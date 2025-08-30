package utils

import (
	"regexp"

	"github.com/fhsmendes/deploy-cloud-run/models"
)

func IsValidCEP(cep string) bool {
	matched, _ := regexp.MatchString(`^\d{8}$`, cep)
	return matched
}

func ConvertTemperatures(celsius float64) models.Temperature {
	fahrenheit := celsius*1.8 + 32
	kelvin := celsius + 273

	return models.Temperature{
		TempC: celsius,
		TempF: fahrenheit,
		TempK: kelvin,
	}
}
