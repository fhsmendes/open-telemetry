package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func initProvider(serviceName, collectorURL string) (func(context.Context) error, error) {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	conn, err := grpc.DialContext(ctx, collectorURL, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(traceProvider)

	otel.SetTextMapPropagator(propagation.TraceContext{})

	return traceProvider.Shutdown, nil
}

type CEPRequest struct {
	CEP string `json:"cep"`
}

type TemperatureResponse struct {
	City  string  `json:"city"`
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

func validateCEP(cep string) bool {
	// Remove qualquer formatação (hífens, espaços)
	cleanCEP := strings.ReplaceAll(cep, "-", "")
	cleanCEP = strings.ReplaceAll(cleanCEP, " ", "")

	// Verifica se tem exatamente 8 dígitos
	matched, _ := regexp.MatchString(`^\d{8}$`, cleanCEP)
	return matched
}

func handleCEPRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := otel.Tracer("service-input-tracer")

	ctx, span := tracer.Start(ctx, "validate-cep")
	defer span.End()

	var req CEPRequest

	// Decodifica o JSON da requisição
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetAttributes(attribute.String("error", "invalid json"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "invalid zipcode"})
		return
	}

	// Adiciona CEP como atributo do span
	span.SetAttributes(attribute.String("cep", req.CEP))

	// Valida o CEP
	if !validateCEP(req.CEP) {
		span.SetAttributes(attribute.String("error", "invalid cep format"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "invalid zipcode"})
		return
	}

	// Limpa o CEP para enviar para o serviço B
	cleanCEP := strings.ReplaceAll(req.CEP, "-", "")
	cleanCEP = strings.ReplaceAll(cleanCEP, " ", "")
	span.End()

	// Chama o serviço B
	ctx, spanServiceB := tracer.Start(ctx, "call-service-b")
	defer spanServiceB.End()

	serviceBURL := os.Getenv("SERVICE_B_URL")
	if serviceBURL == "" {
		log.Fatal("SERVICE_B_URL environment variable not set")
	}

	url := fmt.Sprintf("%s/temperature?cep=%s", serviceBURL, cleanCEP)
	spanServiceB.SetAttributes(
		attribute.String("service.b.url", url),
		attribute.String("clean_cep", cleanCEP),
	)

	// Cria requisição com contexto de tracing
	reqServiceB, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		spanServiceB.SetAttributes(attribute.String("error", "failed to create request"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "internal server error"})
		return
	}

	// Injeta headers de tracing na requisição
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(reqServiceB.Header))

	client := &http.Client{}
	resp, err := client.Do(reqServiceB)
	if err != nil {
		spanServiceB.SetAttributes(attribute.String("error", "service b call failed"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "internal server error"})
		return
	}
	defer resp.Body.Close()

	spanServiceB.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	// Lê a resposta do serviço B
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		spanServiceB.SetAttributes(attribute.String("error", "failed to read response"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "internal server error"})
		return
	}

	// Retorna a resposta do serviço B com o mesmo status code
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func main() {
	// Carrega variáveis de ambiente
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	shutdown, err := initProvider("service-input", os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if err != nil {
		log.Fatalf("failed to initialize tracing provider: %v", err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Fatalf("failed to shutdown tracing provider: %v", err)
		}
	}()

	r := chi.NewRouter()

	// Middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.SetHeader("Content-Type", "application/json"))

	// Rotas
	r.Post("/temperature", handleCEPRequest)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	go func() {
		log.Printf("Service Input running on port %s", port)
		if err := http.ListenAndServe(":"+port, r); err != nil {
			log.Fatal(err)
		}
	}()

	select {
	case <-sigCh:
		log.Println("Shutting down gracefully...")
		cancel()
	case <-ctx.Done():
		log.Println("Shutting down due to other reason...")
	}

	_, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
}
