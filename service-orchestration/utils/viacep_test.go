package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/fhsmendes/deploy-cloud-run/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockHTTPClient implementa http.Client para testes
type MockHTTPClient struct {
	responses map[string]*http.Response
	errors    map[string]error
}

func NewMockHTTPClient() *MockHTTPClient {
	return &MockHTTPClient{
		responses: make(map[string]*http.Response),
		errors:    make(map[string]error),
	}
}

func (m *MockHTTPClient) AddResponse(url string, statusCode int, body string) {
	m.responses[url] = &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func (m *MockHTTPClient) AddError(url string, err error) {
	m.errors[url] = err
}

func (m *MockHTTPClient) Get(url string) (*http.Response, error) {
	if err, exists := m.errors[url]; exists {
		return nil, err
	}

	if resp, exists := m.responses[url]; exists {
		return resp, nil
	}

	return &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(strings.NewReader("Not Found")),
		Header:     make(http.Header),
	}, nil
}

// Variável global para injetar mock HTTP client
var httpClient HTTPGetter = http.DefaultClient

type HTTPGetter interface {
	Get(url string) (*http.Response, error)
}

// Função para substituir o cliente HTTP (usada nos testes)
func SetHTTPClient(client HTTPGetter) {
	if client == nil {
		httpClient = http.DefaultClient
	} else {
		httpClient = client
	}
}

// Versão testável da função GetCityFromCEP que usa httpClient injetável
func GetCityFromCEPWithClient(cep string, client HTTPGetter) (string, error) {
	url := fmt.Sprintf(UrlViaCEP, cep)
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var viaCEP models.ViaCEP
	if err := json.NewDecoder(resp.Body).Decode(&viaCEP); err != nil {
		return "", err
	}

	if viaCEP.Erro || viaCEP.Localidade == "" {
		return "", fmt.Errorf("zipcode not found")
	}

	return viaCEP.Localidade, nil
}

func TestGetCityFromCEP_Success(t *testing.T) {
	mockClient := NewMockHTTPClient()
	expectedURL := "https://viacep.com.br/ws/01001000/json/"
	mockResponse := `{"localidade": "São Paulo", "uf": "SP"}`
	mockClient.AddResponse(expectedURL, 200, mockResponse)

	city, err := GetCityFromCEPWithClient("01001000", mockClient)

	require.NoError(t, err)
	assert.Equal(t, "São Paulo", city)
}

func TestGetCityFromCEP_CEPNotFound(t *testing.T) {
	mockClient := NewMockHTTPClient()
	expectedURL := "https://viacep.com.br/ws/99999999/json/"
	mockResponse := `{"erro": true}`
	mockClient.AddResponse(expectedURL, 200, mockResponse)

	city, err := GetCityFromCEPWithClient("99999999", mockClient)

	require.Error(t, err)
	assert.Equal(t, "", city)
	assert.Equal(t, "zipcode not found", err.Error())
}

func TestGetCityFromCEP_EmptyLocalidade(t *testing.T) {
	mockClient := NewMockHTTPClient()
	expectedURL := "https://viacep.com.br/ws/12345678/json/"
	mockResponse := `{"localidade": "", "uf": "SP"}`
	mockClient.AddResponse(expectedURL, 200, mockResponse)

	city, err := GetCityFromCEPWithClient("12345678", mockClient)

	require.Error(t, err)
	assert.Equal(t, "", city)
	assert.Equal(t, "zipcode not found", err.Error())
}

func TestGetCityFromCEP_HTTPError(t *testing.T) {
	mockClient := NewMockHTTPClient()
	expectedURL := "https://viacep.com.br/ws/01001000/json/"
	mockClient.AddError(expectedURL, errors.New("network error"))

	city, err := GetCityFromCEPWithClient("01001000", mockClient)

	require.Error(t, err)
	assert.Equal(t, "", city)
	assert.Equal(t, "network error", err.Error())
}

func TestGetCityFromCEP_InvalidJSON(t *testing.T) {
	mockClient := NewMockHTTPClient()
	expectedURL := "https://viacep.com.br/ws/01001000/json/"
	mockResponse := `{"invalid": json}`
	mockClient.AddResponse(expectedURL, 200, mockResponse)

	city, err := GetCityFromCEPWithClient("01001000", mockClient)

	require.Error(t, err)
	assert.Equal(t, "", city)
	assert.Contains(t, err.Error(), "invalid character")
}

func TestGetCityFromCEP_MultipleValidCEPs(t *testing.T) {
	testCases := []struct {
		name         string
		cep          string
		mockResponse string
		expectedCity string
	}{
		{
			name:         "São Paulo",
			cep:          "01001000",
			mockResponse: `{"localidade": "São Paulo", "uf": "SP"}`,
			expectedCity: "São Paulo",
		},
		{
			name:         "Rio de Janeiro",
			cep:          "20040020",
			mockResponse: `{"localidade": "Rio de Janeiro", "uf": "RJ"}`,
			expectedCity: "Rio de Janeiro",
		},
		{
			name:         "Belo Horizonte",
			cep:          "30112000",
			mockResponse: `{"localidade": "Belo Horizonte", "uf": "MG"}`,
			expectedCity: "Belo Horizonte",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := NewMockHTTPClient()
			expectedURL := fmt.Sprintf("https://viacep.com.br/ws/%s/json/", tc.cep)
			mockClient.AddResponse(expectedURL, 200, tc.mockResponse)

			city, err := GetCityFromCEPWithClient(tc.cep, mockClient)

			require.NoError(t, err)
			assert.Equal(t, tc.expectedCity, city)
		})
	}
}

func TestGetCityFromCEP_ErrorScenarios(t *testing.T) {
	testCases := []struct {
		name          string
		cep           string
		mockResponse  string
		statusCode    int
		shouldError   bool
		expectedError string
	}{
		{
			name:          "Error field true",
			cep:           "00000000",
			mockResponse:  `{"erro": true}`,
			statusCode:    200,
			shouldError:   true,
			expectedError: "zipcode not found",
		},
		{
			name:          "Both erro and empty localidade",
			cep:           "11111111",
			mockResponse:  `{"erro": true, "localidade": ""}`,
			statusCode:    200,
			shouldError:   true,
			expectedError: "zipcode not found",
		},
		{
			name:          "Valid response with all fields",
			cep:           "01310100",
			mockResponse:  `{"cep": "01310-100", "logradouro": "Avenida Paulista", "complemento": "", "bairro": "Bela Vista", "localidade": "São Paulo", "uf": "SP", "ibge": "3550308", "gia": "1004", "ddd": "11", "siafi": "7107"}`,
			statusCode:    200,
			shouldError:   false,
			expectedError: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := NewMockHTTPClient()
			expectedURL := fmt.Sprintf("https://viacep.com.br/ws/%s/json/", tc.cep)
			mockClient.AddResponse(expectedURL, tc.statusCode, tc.mockResponse)

			city, err := GetCityFromCEPWithClient(tc.cep, mockClient)

			if tc.shouldError {
				require.Error(t, err)
				assert.Equal(t, "", city)
				if tc.expectedError != "" {
					assert.Equal(t, tc.expectedError, err.Error())
				}
			} else {
				require.NoError(t, err)
				assert.NotEqual(t, "", city)
			}
		})
	}
}

func TestGetCityFromCEP_URLFormatting(t *testing.T) {
	mockClient := NewMockHTTPClient()

	testCEPs := []string{"01001000", "12345678", "87654321"}

	for _, cep := range testCEPs {
		expectedURL := fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cep)
		mockResponse := fmt.Sprintf(`{"localidade": "Test City %s", "uf": "SP"}`, cep)
		mockClient.AddResponse(expectedURL, 200, mockResponse)

		city, err := GetCityFromCEPWithClient(cep, mockClient)

		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("Test City %s", cep), city)
	}
}

func TestGetCityFromCEP_ResponseBodyHandling(t *testing.T) {
	t.Run("Empty response body", func(t *testing.T) {
		mockClient := NewMockHTTPClient()
		expectedURL := "https://viacep.com.br/ws/01001000/json/"
		mockClient.AddResponse(expectedURL, 200, "")

		city, err := GetCityFromCEPWithClient("01001000", mockClient)

		require.Error(t, err)
		assert.Equal(t, "", city)
	})

	t.Run("Large response body", func(t *testing.T) {
		mockClient := NewMockHTTPClient()
		expectedURL := "https://viacep.com.br/ws/01001000/json/"
		// Simulate a large but valid JSON response
		mockResponse := `{"cep": "01001-000", "logradouro": "Praça da Sé", "complemento": "lado ímpar", "bairro": "Sé", "localidade": "São Paulo", "uf": "SP", "ibge": "3550308", "gia": "1004", "ddd": "11", "siafi": "7107", "extra_field": "` + strings.Repeat("a", 1000) + `"}`
		mockClient.AddResponse(expectedURL, 200, mockResponse)

		city, err := GetCityFromCEPWithClient("01001000", mockClient)

		require.NoError(t, err)
		assert.Equal(t, "São Paulo", city)
	})
}
