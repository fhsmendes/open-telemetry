package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/fhsmendes/deploy-cloud-run/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
)

func TemperatureHandler(w http.ResponseWriter, r *http.Request) {
	// Extract tracing context from HTTP headers
	ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	tracer := otel.Tracer("service-orchestration")

	ctx, mainSpan := tracer.Start(ctx, "temperature-handler")
	defer mainSpan.End()

	cep := r.URL.Query().Get("cep")
	mainSpan.SetAttributes(attribute.String("cep", cep))

	fmt.Println("Received request for zipcode:", cep)

	if !utils.IsValidCEP(cep) {
		fmt.Println("Invalid zipcode:", cep)
		mainSpan.SetStatus(codes.Error, "invalid zipcode")
		mainSpan.SetAttributes(attribute.Bool("valid_cep", false))
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte("invalid zipcode"))
		return
	}

	fmt.Println("Valid zipcode:", cep)
	mainSpan.SetAttributes(attribute.Bool("valid_cep", true))

	ctx, spanCity := tracer.Start(ctx, "get-city-from-cep")
	spanCity.SetAttributes(attribute.String("cep", cep))

	city, err := utils.GetCityFromCEP(ctx, cep, spanCity)
	if err != nil {
		fmt.Println("Error getting city from zipcode:", err)
		spanCity.SetStatus(codes.Error, "city not found")
		spanCity.End()
		mainSpan.SetStatus(codes.Error, "can not find zipcode")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("can not find zipcode"))
		return
	}
	spanCity.SetAttributes(attribute.String("city", city))
	spanCity.SetStatus(codes.Ok, "city found successfully")
	spanCity.End()

	fmt.Println("City found:", city)

	ctx, spanTemp := tracer.Start(ctx, "get-temperature-from-weather-api")
	spanTemp.SetAttributes(attribute.String("city", city))

	tempC, err := utils.GetTemperature(ctx, city, spanTemp)
	if err != nil {
		fmt.Println("Error getting temperature:", err)
		spanTemp.SetStatus(codes.Error, "failed to get temperature")
		spanTemp.End()
		mainSpan.SetStatus(codes.Error, "error getting temperature")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error getting temperature"))
		return
	}
	spanTemp.SetAttributes(attribute.Float64("temperature_celsius", tempC))
	spanTemp.SetStatus(codes.Ok, "temperature retrieved successfully")
	spanTemp.End()

	fmt.Println("Temperature in Celsius:", tempC)

	_, spanConvert := tracer.Start(ctx, "convert-temperatures")
	temps := utils.ConvertTemperatures(tempC)
	temps.City = city
	spanConvert.SetAttributes(
		attribute.Float64("temp_celsius", temps.TempC),
		attribute.Float64("temp_fahrenheit", temps.TempF),
		attribute.Float64("temp_kelvin", temps.TempK),
	)
	spanConvert.SetStatus(codes.Ok, "temperatures converted successfully")
	spanConvert.End()

	fmt.Println("Converted temperatures:", temps)

	mainSpan.SetAttributes(attribute.String("response_city", city))
	mainSpan.SetStatus(codes.Ok, "request processed successfully")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(temps)
}
