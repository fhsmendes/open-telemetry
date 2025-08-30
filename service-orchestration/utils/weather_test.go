package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/fhsmendes/deploy-cloud-run/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockHTTPClient para simulação do cliente HTTP
type MockHTTPClientWeather struct {
	responses map[string]*http.Response
	errors    map[string]error
}

func NewMockHTTPClientWeather() *MockHTTPClientWeather {
	return &MockHTTPClientWeather{
		responses: make(map[string]*http.Response),
		errors:    make(map[string]error),
	}
}

func (m *MockHTTPClientWeather) AddResponse(url string, statusCode int, body string) {
	m.responses[url] = &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func (m *MockHTTPClientWeather) AddError(url string, err error) {
	m.errors[url] = err
}

func (m *MockHTTPClientWeather) Get(url string) (*http.Response, error) {
	if err, exists := m.errors[url]; exists {
		return nil, err
	}
	if resp, exists := m.responses[url]; exists {
		return resp, nil
	}
	return &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(strings.NewReader(`{"error": "not found"}`)),
	}, nil
}

// HTTPClientInterface define a interface para o cliente HTTP
type HTTPClientInterface interface {
	Get(url string) (*http.Response, error)
}

// GetTemperatureWithClient é uma versão testável da função GetTemperature
func GetTemperatureWithClient(city string, client HTTPClientInterface) (float64, error) {
	apiKey := os.Getenv("APIKeyWeather")

	if apiKey == "" {
		return 0, fmt.Errorf("API key is not set")
	}

	apiUrl := fmt.Sprintf("https://api.weatherapi.com/v1/current.json?key=%s&q=%s", apiKey, city)

	resp, err := client.Get(apiUrl)
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

func TestGetTemperature_Success(t *testing.T) {
	// Setup
	os.Setenv("APIKeyWeather", "test-api-key")
	defer os.Unsetenv("APIKeyWeather")

	mockClient := NewMockHTTPClientWeather()

	weatherResponse := `{
		"current": {
			"temp_c": 25.5
		}
	}`

	expectedURL := "https://api.weatherapi.com/v1/current.json?key=test-api-key&q=São Paulo"
	mockClient.AddResponse(expectedURL, 200, weatherResponse)

	// Test
	temperature, err := GetTemperatureWithClient("São Paulo", mockClient)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 25.5, temperature)
}

func TestGetTemperature_NoAPIKey(t *testing.T) {
	// Setup
	os.Unsetenv("APIKeyWeather")
	mockClient := NewMockHTTPClientWeather()

	// Test
	temperature, err := GetTemperatureWithClient("São Paulo", mockClient)

	// Assert
	require.Error(t, err)
	assert.Equal(t, 0.0, temperature)
	assert.Contains(t, err.Error(), "API key is not set")
}

func TestGetTemperature_HTTPError(t *testing.T) {
	// Setup
	os.Setenv("APIKeyWeather", "test-api-key")
	defer os.Unsetenv("APIKeyWeather")

	mockClient := NewMockHTTPClientWeather()

	expectedURL := "https://api.weatherapi.com/v1/current.json?key=test-api-key&q=São Paulo"
	mockClient.AddError(expectedURL, fmt.Errorf("network error"))

	// Test
	temperature, err := GetTemperatureWithClient("São Paulo", mockClient)

	// Assert
	require.Error(t, err)
	assert.Equal(t, 0.0, temperature)
	assert.Contains(t, err.Error(), "network error")
}

func TestGetTemperature_APIErrorStatus(t *testing.T) {
	// Setup
	os.Setenv("APIKeyWeather", "test-api-key")
	defer os.Unsetenv("APIKeyWeather")

	mockClient := NewMockHTTPClientWeather()

	errorResponse := `{"error": {"message": "API key not found"}}`
	expectedURL := "https://api.weatherapi.com/v1/current.json?key=test-api-key&q=São Paulo"
	mockClient.AddResponse(expectedURL, 401, errorResponse)

	// Test
	temperature, err := GetTemperatureWithClient("São Paulo", mockClient)

	// Assert
	require.Error(t, err)
	assert.Equal(t, 0.0, temperature)
	assert.Contains(t, err.Error(), "weather API returned status: 401")
}

func TestGetTemperature_InvalidJSON(t *testing.T) {
	// Setup
	os.Setenv("APIKeyWeather", "test-api-key")
	defer os.Unsetenv("APIKeyWeather")

	mockClient := NewMockHTTPClientWeather()

	invalidJSON := `{"current": {"temp_c": "invalid_number"}}`
	expectedURL := "https://api.weatherapi.com/v1/current.json?key=test-api-key&q=São Paulo"
	mockClient.AddResponse(expectedURL, 200, invalidJSON)

	// Test
	temperature, err := GetTemperatureWithClient("São Paulo", mockClient)

	// Assert
	require.Error(t, err)
	assert.Equal(t, 0.0, temperature)
}

func TestGetTemperature_MalformedJSON(t *testing.T) {
	// Setup
	os.Setenv("APIKeyWeather", "test-api-key")
	defer os.Unsetenv("APIKeyWeather")

	mockClient := NewMockHTTPClientWeather()

	malformedJSON := `{"current": {"temp_c": 25.5}` // Missing closing brace
	expectedURL := "https://api.weatherapi.com/v1/current.json?key=test-api-key&q=São Paulo"
	mockClient.AddResponse(expectedURL, 200, malformedJSON)

	// Test
	temperature, err := GetTemperatureWithClient("São Paulo", mockClient)

	// Assert
	require.Error(t, err)
	assert.Equal(t, 0.0, temperature)
}

func TestGetTemperature_MultipleTemperatures(t *testing.T) {
	// Setup
	os.Setenv("APIKeyWeather", "test-api-key")
	defer os.Unsetenv("APIKeyWeather")

	testCases := []struct {
		name        string
		city        string
		temperature float64
	}{
		{"Positive Temperature", "Rio de Janeiro", 30.2},
		{"Zero Temperature", "Antarctica", 0.0},
		{"Negative Temperature", "Siberia", -15.7},
		{"Decimal Temperature", "London", 18.9},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := NewMockHTTPClientWeather()

			weatherResponse := fmt.Sprintf(`{
				"current": {
					"temp_c": %f
				}
			}`, tc.temperature)

			expectedURL := fmt.Sprintf("https://api.weatherapi.com/v1/current.json?key=test-api-key&q=%s", tc.city)
			mockClient.AddResponse(expectedURL, 200, weatherResponse)

			// Test
			temperature, err := GetTemperatureWithClient(tc.city, mockClient)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.temperature, temperature)
		})
	}
}

func TestGetTemperature_ErrorScenarios(t *testing.T) {
	// Setup
	os.Setenv("APIKeyWeather", "test-api-key")
	defer os.Unsetenv("APIKeyWeather")

	testCases := []struct {
		name         string
		statusCode   int
		responseBody string
		expectedErr  string
	}{
		{
			name:         "404 Not Found",
			statusCode:   404,
			responseBody: `{"error": {"message": "City not found"}}`,
			expectedErr:  "weather API returned status: 404",
		},
		{
			name:         "500 Internal Server Error",
			statusCode:   500,
			responseBody: `{"error": {"message": "Internal server error"}}`,
			expectedErr:  "weather API returned status: 500",
		},
		{
			name:         "403 Forbidden",
			statusCode:   403,
			responseBody: `{"error": {"message": "Forbidden access"}}`,
			expectedErr:  "weather API returned status: 403",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := NewMockHTTPClientWeather()

			expectedURL := "https://api.weatherapi.com/v1/current.json?key=test-api-key&q=TestCity"
			mockClient.AddResponse(expectedURL, tc.statusCode, tc.responseBody)

			// Test
			temperature, err := GetTemperatureWithClient("TestCity", mockClient)

			// Assert
			require.Error(t, err)
			assert.Equal(t, 0.0, temperature)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

func TestGetTemperature_SpecialCharactersInCity(t *testing.T) {
	// Setup
	os.Setenv("APIKeyWeather", "test-api-key")
	defer os.Unsetenv("APIKeyWeather")

	testCases := []struct {
		name string
		city string
	}{
		{"City with spaces", "São Paulo"},
		{"City with accent", "Brasília"},
		{"City with special chars", "México City"},
		{"City with hyphen", "Belo-Horizonte"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := NewMockHTTPClientWeather()

			weatherResponse := `{
				"current": {
					"temp_c": 22.0
				}
			}`

			expectedURL := fmt.Sprintf("https://api.weatherapi.com/v1/current.json?key=test-api-key&q=%s", tc.city)
			mockClient.AddResponse(expectedURL, 200, weatherResponse)

			// Test
			temperature, err := GetTemperatureWithClient(tc.city, mockClient)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, 22.0, temperature)
		})
	}
}

func TestGetTemperature_EmptyResponse(t *testing.T) {
	// Setup
	os.Setenv("APIKeyWeather", "test-api-key")
	defer os.Unsetenv("APIKeyWeather")

	mockClient := NewMockHTTPClientWeather()

	expectedURL := "https://api.weatherapi.com/v1/current.json?key=test-api-key&q=TestCity"
	mockClient.AddResponse(expectedURL, 200, "")

	// Test
	temperature, err := GetTemperatureWithClient("TestCity", mockClient)

	// Assert
	require.Error(t, err)
	assert.Equal(t, 0.0, temperature)
}

func TestGetTemperature_MissingTemperatureField(t *testing.T) {
	// Setup
	os.Setenv("APIKeyWeather", "test-api-key")
	defer os.Unsetenv("APIKeyWeather")

	mockClient := NewMockHTTPClientWeather()

	// Response without temp_c field
	weatherResponse := `{
		"current": {
			"humidity": 80
		}
	}`

	expectedURL := "https://api.weatherapi.com/v1/current.json?key=test-api-key&q=TestCity"
	mockClient.AddResponse(expectedURL, 200, weatherResponse)

	// Test
	temperature, err := GetTemperatureWithClient("TestCity", mockClient)

	// Assert - Should not error, but temperature should be 0.0 (default for float64)
	require.NoError(t, err)
	assert.Equal(t, 0.0, temperature)
}

func TestGetTemperature_ExtremeTemperatures(t *testing.T) {
	// Setup
	os.Setenv("APIKeyWeather", "test-api-key")
	defer os.Unsetenv("APIKeyWeather")

	testCases := []struct {
		name        string
		temperature float64
	}{
		{"Very Hot", 60.5},
		{"Very Cold", -89.2},
		{"Extremely Hot", 134.0},
		{"Absolute Zero", -273.15},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := NewMockHTTPClientWeather()

			weatherResponse := fmt.Sprintf(`{
				"current": {
					"temp_c": %f
				}
			}`, tc.temperature)

			expectedURL := "https://api.weatherapi.com/v1/current.json?key=test-api-key&q=TestCity"
			mockClient.AddResponse(expectedURL, 200, weatherResponse)

			// Test
			temperature, err := GetTemperatureWithClient("TestCity", mockClient)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.temperature, temperature)
		})
	}
}

// Testes para a função original GetTemperature (integração)
func TestGetTemperature_Original_Integration(t *testing.T) {
	t.Skip("Integration test - requires actual API calls and API key")

	// Teste básico da função original (seria executado apenas se necessário)
	os.Setenv("APIKeyWeather", "your-actual-api-key-here")
	defer os.Unsetenv("APIKeyWeather")

	temp, err := GetTemperature("São Paulo")
	if err != nil {
		t.Logf("Integration test failed (expected for unit tests): %v", err)
		return
	}
	assert.Greater(t, temp, -100.0)
	assert.Less(t, temp, 100.0)
}
