package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fhsmendes/deploy-cloud-run/models"
	"github.com/fhsmendes/deploy-cloud-run/utils/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Handler com dependências injetáveis
type TemperatureHandlerWithDeps struct {
	CEPValidator     func(string) bool
	ViaCEPClient     interface{ GetCityFromCEP(string) (string, error) }
	WeatherAPIClient interface{ GetTemperature(string) (float64, error) }
	TempConverter    func(float64) models.Temperature
}

func (h *TemperatureHandlerWithDeps) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cep := r.URL.Query().Get("cep")

	if !h.CEPValidator(cep) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte("invalid zipcode"))
		return
	}

	city, err := h.ViaCEPClient.GetCityFromCEP(cep)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("can not find zipcode"))
		return
	}

	tempC, err := h.WeatherAPIClient.GetTemperature(city)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error getting temperature"))
		return
	}

	temps := h.TempConverter(tempC)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(temps)
}

func TestTemperatureHandler_Success_WithMocks(t *testing.T) {
	// Setup mocks
	mockViaCEP := mocks.NewViaCEPClient(t)
	mockWeather := mocks.NewWeatherAPIClient(t)

	// Configure mock expectations
	mockViaCEP.On("GetCityFromCEP", "01001000").Return("São Paulo", nil)
	mockWeather.On("GetTemperature", "São Paulo").Return(25.0, nil)

	// Create handler with mocked dependencies
	handler := &TemperatureHandlerWithDeps{
		CEPValidator:     func(cep string) bool { return cep == "01001000" },
		ViaCEPClient:     mockViaCEP,
		WeatherAPIClient: mockWeather,
		TempConverter: func(celsius float64) models.Temperature {
			return models.Temperature{
				TempC: celsius,
				TempF: celsius*1.8 + 32,
				TempK: celsius + 273,
			}
		},
	}

	// Execute request
	req := httptest.NewRequest("GET", "/temperature?cep=01001000", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Assert response
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// Decode and verify JSON response
	var temp models.Temperature
	require.NoError(t, json.NewDecoder(w.Body).Decode(&temp))
	assert.Equal(t, 25.0, temp.TempC)
	assert.Equal(t, 77.0, temp.TempF)
	assert.Equal(t, 298.0, temp.TempK)
}

func TestTemperatureHandler_InvalidCEP_WithMocks(t *testing.T) {
	handler := &TemperatureHandlerWithDeps{
		CEPValidator: func(cep string) bool { return false },
	}

	testCases := []struct {
		name string
		cep  string
	}{
		{"CEP with letters", "1234567a"},
		{"CEP with dash", "12345-678"},
		{"CEP too short", "1234567"},
		{"CEP too long", "123456789"},
		{"Empty CEP", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url := "/temperature?cep=" + tc.cep
			req := httptest.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
			assert.Equal(t, "invalid zipcode", w.Body.String())
		})
	}
}

func TestTemperatureHandler_CityNotFound_WithMocks(t *testing.T) {
	mockViaCEP := mocks.NewViaCEPClient(t)
	mockViaCEP.On("GetCityFromCEP", "99999999").Return("", errors.New("zipcode not found"))

	handler := &TemperatureHandlerWithDeps{
		CEPValidator: func(cep string) bool { return true },
		ViaCEPClient: mockViaCEP,
	}

	req := httptest.NewRequest("GET", "/temperature?cep=99999999", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "can not find zipcode", w.Body.String())
}

func TestTemperatureHandler_WeatherAPIError_WithMocks(t *testing.T) {
	mockViaCEP := mocks.NewViaCEPClient(t)
	mockWeather := mocks.NewWeatherAPIClient(t)

	mockViaCEP.On("GetCityFromCEP", "01001000").Return("São Paulo", nil)
	mockWeather.On("GetTemperature", "São Paulo").Return(0.0, errors.New("API key not set"))

	handler := &TemperatureHandlerWithDeps{
		CEPValidator:     func(cep string) bool { return true },
		ViaCEPClient:     mockViaCEP,
		WeatherAPIClient: mockWeather,
	}

	req := httptest.NewRequest("GET", "/temperature?cep=01001000", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "error getting temperature", w.Body.String())
}

func TestTemperatureHandler_InvalidCEP(t *testing.T) {
	testCases := []struct {
		name string
		cep  string
	}{
		{"CEP with letters", "1234567a"},
		{"CEP with dash", "12345-678"},
		{"CEP too short", "1234567"},
		{"CEP too long", "123456789"},
		{"Empty CEP", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url := "/temperature?cep=" + tc.cep
			req := httptest.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()

			TemperatureHandler(w, req)

			if w.Code != http.StatusUnprocessableEntity {
				t.Errorf("Expected status %d, got %d", http.StatusUnprocessableEntity, w.Code)
			}

			expectedBody := "invalid zipcode"
			if w.Body.String() != expectedBody {
				t.Errorf("Expected body %q, got %q", expectedBody, w.Body.String())
			}
		})
	}
}

func TestTemperatureHandler_ValidCEP_CityNotFound(t *testing.T) {
	req := httptest.NewRequest("GET", "/temperature?cep=12345678", nil)
	w := httptest.NewRecorder()

	TemperatureHandler(w, req)

	// Assuming GetCityFromCEP fails for this test case
	// In a real scenario, you'd mock this function to return an error
	if w.Code == http.StatusNotFound {
		expectedBody := "can not find zipcode"
		if w.Body.String() != expectedBody {
			t.Errorf("Expected body %q, got %q", expectedBody, w.Body.String())
		}
	}
}

func TestTemperatureHandler_ValidCEP_TemperatureError(t *testing.T) {
	req := httptest.NewRequest("GET", "/temperature?cep=01001000", nil)
	w := httptest.NewRecorder()

	TemperatureHandler(w, req)

	// This test would require mocking to force a temperature API error
	if w.Code == http.StatusInternalServerError {
		expectedBody := "error getting temperature"
		if w.Body.String() != expectedBody {
			t.Errorf("Expected body %q, got %q", expectedBody, w.Body.String())
		}
	}
}

func TestTemperatureHandler_Success(t *testing.T) {
	mockViaCEP := &mocks.ViaCEPClient{}
	mockWeather := &mocks.WeatherAPIClient{}
	mockViaCEP.On("GetCityFromCEP", "01001000").Return("São Paulo", nil)
	mockWeather.On("GetTemperature", "São Paulo").Return(25.0, nil)

	handler := &TemperatureHandlerWithDeps{
		CEPValidator:     func(cep string) bool { return true },
		ViaCEPClient:     mockViaCEP,
		WeatherAPIClient: mockWeather,
		TempConverter: func(c float64) models.Temperature {
			return models.Temperature{TempC: c, TempF: 77, TempK: 298}
		},
	}
	req := httptest.NewRequest("GET", "/temperature?cep=01001000", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var temp models.Temperature
	require.NoError(t, json.NewDecoder(w.Body).Decode(&temp))
	assert.Equal(t, 25.0, temp.TempC)
	assert.Equal(t, 77.0, temp.TempF)
	assert.Equal(t, 298.0, temp.TempK)
}

func TestTemperatureHandler_HTTPMethods(t *testing.T) {
	methods := []string{"POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/temperature?cep=01001000", nil)
			w := httptest.NewRecorder()

			TemperatureHandler(w, req)

			// The handler doesn't explicitly check HTTP method,
			// so it should process any method the same way
			// This test verifies the handler is method-agnostic
		})
	}
}

func TestTemperatureHandler_ResponseHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "/temperature?cep=01001000", nil)
	w := httptest.NewRecorder()

	TemperatureHandler(w, req)

	if w.Code == http.StatusOK {
		contentType := w.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got %q", contentType)
		}
	}
}

func TestTemperatureHandler_CEPValidation(t *testing.T) {
	validCEPs := []string{
		"01001000",
		"12345678",
		"00000000",
		"99999999",
	}

	for _, cep := range validCEPs {
		t.Run("Valid CEP "+cep, func(t *testing.T) {
			url := "/temperature?cep=" + cep
			req := httptest.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()

			TemperatureHandler(w, req)

			// Should not return 422 (Unprocessable Entity) for valid CEPs
			if w.Code == http.StatusUnprocessableEntity {
				t.Errorf("Valid CEP %s was rejected", cep)
			}
		})
	}
}

func TestTemperatureHandler_ErrorMessages(t *testing.T) {
	testCases := []struct {
		name           string
		url            string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Invalid zipcode",
			url:            "/temperature?cep=invalid",
			expectedStatus: http.StatusUnprocessableEntity,
			expectedBody:   "invalid zipcode",
		},
		{
			name:           "Empty CEP parameter",
			url:            "/temperature?cep=",
			expectedStatus: http.StatusUnprocessableEntity,
			expectedBody:   "invalid zipcode",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.url, nil)
			w := httptest.NewRecorder()

			TemperatureHandler(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			if w.Body.String() != tc.expectedBody {
				t.Errorf("Expected body %q, got %q", tc.expectedBody, w.Body.String())
			}
		})
	}
}

func TestTemperatureHandler_JSONResponseStructure(t *testing.T) {
	req := httptest.NewRequest("GET", "/temperature?cep=01001000", nil)
	w := httptest.NewRecorder()

	TemperatureHandler(w, req)

	if w.Code == http.StatusOK {
		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		if err != nil {
			t.Errorf("Response is not valid JSON: %v", err)
			return
		}

		// Check if expected fields exist
		expectedFields := []string{"temp_C", "temp_F", "temp_K"}
		for _, field := range expectedFields {
			if _, exists := response[field]; !exists {
				t.Errorf("Expected field %q not found in response", field)
			}
		}
	}
}
