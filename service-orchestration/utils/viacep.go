package utils

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/fhsmendes/deploy-cloud-run/models"
)

const UrlViaCEP = "https://viacep.com.br/ws/%s/json/"

type ViaCEPClient interface {
	GetCityFromCEP(cep string) (string, error)
}

func GetCityFromCEP(cep string) (string, error) {
	url := fmt.Sprintf(UrlViaCEP, cep)
	resp, err := http.Get(url)
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
