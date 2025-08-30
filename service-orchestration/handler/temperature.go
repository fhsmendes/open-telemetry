package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/fhsmendes/deploy-cloud-run/utils"
)

func TemperatureHandler(w http.ResponseWriter, r *http.Request) {
	cep := r.URL.Query().Get("cep")

	fmt.Println("Received request for zipcode:", cep)

	if !utils.IsValidCEP(cep) {
		fmt.Println("Invalid zipcode:", cep)
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte("invalid zipcode"))
		return
	}

	fmt.Println("Valid zipcode:", cep)

	city, err := utils.GetCityFromCEP(cep)
	if err != nil {
		fmt.Println("Error getting city from zipcode:", err)
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("can not find zipcode"))
		return
	}

	fmt.Println("City found:", city)

	tempC, err := utils.GetTemperature(city)
	if err != nil {
		fmt.Println("Error getting temperature:", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error getting temperature"))
		return
	}

	fmt.Println("Temperature in Celsius:", tempC)
	temps := utils.ConvertTemperatures(tempC)
	temps.City = city
	fmt.Println("Converted temperatures:", temps)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(temps)
}
