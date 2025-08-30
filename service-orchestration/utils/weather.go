package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/fhsmendes/deploy-cloud-run/models"
)

const UrlWeatherAPI = "https://api.weatherapi.com/v1/current.json?key=%s&q=%s"

type WeatherAPIClient interface {
	GetTemperature(city string) (float64, error)
}

func GetTemperature(city string) (float64, error) {
	apiKey := os.Getenv("APIKeyWeather")

	if apiKey == "" {
		return 0, fmt.Errorf("API key is not set")
	}

	encodedCity := url.QueryEscape(city)
	fmt.Println("Encoded city:", encodedCity)

	apiUrl := fmt.Sprintf(UrlWeatherAPI, apiKey, encodedCity)

	resp, err := http.Get(apiUrl)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("weather API returned status: %d", resp.StatusCode)
	}

	var weather models.WeatherAPI
	if err := json.NewDecoder(resp.Body).Decode(&weather); err != nil {
		return 0, err
	}

	return weather.Current.TempC, nil
}
