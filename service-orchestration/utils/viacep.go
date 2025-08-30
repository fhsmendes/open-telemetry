package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/fhsmendes/deploy-cloud-run/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const UrlViaCEP = "https://viacep.com.br/ws/%s/json/"

type ViaCEPClient interface {
	GetCityFromCEP(ctx context.Context, cep string, span trace.Span) (string, error)
}

func GetCityFromCEP(ctx context.Context, cep string, span trace.Span) (string, error) {
	url := fmt.Sprintf(UrlViaCEP, cep)

	span.SetAttributes(
		attribute.String("viacep.url", url),
		attribute.String("viacep.cep", cep),
		attribute.String("http.method", "GET"),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create HTTP request")
		return "", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "HTTP request failed")
		return "", err
	}
	defer resp.Body.Close()

	span.SetAttributes(
		attribute.Int("http.status_code", resp.StatusCode),
		attribute.String("http.status", resp.Status),
	)

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("ViaCEP API returned status: %d", resp.StatusCode)
		span.RecordError(err)
		span.SetStatus(codes.Error, "unexpected HTTP status code")
		return "", err
	}

	var viaCEP models.ViaCEP
	if err := json.NewDecoder(resp.Body).Decode(&viaCEP); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to decode JSON response")
		return "", err
	}

	span.SetAttributes(
		attribute.String("viacep.localidade", viaCEP.Localidade),
		attribute.Bool("viacep.erro", viaCEP.Erro),
	)

	if viaCEP.Erro || viaCEP.Localidade == "" {
		err := fmt.Errorf("can not find zipcode")
		span.RecordError(err)
		span.SetStatus(codes.Error, "can not find zipcodeP")
		return "", err
	}

	span.SetStatus(codes.Ok, "city successfully retrieved from ViaCEP")
	return viaCEP.Localidade, nil
}
