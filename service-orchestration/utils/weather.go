package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/fhsmendes/deploy-cloud-run/models"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const UrlWeatherAPI = "https://api.weatherapi.com/v1/current.json?key=%s&q=%s"

type WeatherAPIClient interface {
	GetTemperature(ctx context.Context, city string, span trace.Span) (float64, error)
}

func GetTemperature(ctx context.Context, city string, span trace.Span) (float64, error) {
	apiKey := os.Getenv("APIKeyWeather")

	if apiKey == "" {
		span.RecordError(fmt.Errorf("API key is not set"))
		span.SetStatus(codes.Error, "API key is not set")
		return 0, fmt.Errorf("API key is not set")
	}

	encodedCity := url.QueryEscape(city)
	fmt.Println("Encoded city:", encodedCity)

	apiUrl := fmt.Sprintf(UrlWeatherAPI, apiKey, encodedCity)
	req, err := http.NewRequestWithContext(ctx, "GET", apiUrl, nil)
	if err != nil {
		span.RecordError(fmt.Errorf("failed to create request: %w", err))
		span.SetStatus(codes.Error, "failed to create request")
		return 0, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(fmt.Errorf("failed to get temperature: %w", err))
		span.SetStatus(codes.Error, "failed to get temperature")
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		span.RecordError(fmt.Errorf("weather API returned status: %d", resp.StatusCode))
		span.SetStatus(codes.Error, "weather API returned error status")
		return 0, fmt.Errorf("weather API returned status: %d", resp.StatusCode)
	}

	var weather models.WeatherAPI
	if err := json.NewDecoder(resp.Body).Decode(&weather); err != nil {
		span.RecordError(fmt.Errorf("failed to decode response: %w", err))
		span.SetStatus(codes.Error, "failed to decode response")
		return 0, err
	}

	return weather.Current.TempC, nil
}
