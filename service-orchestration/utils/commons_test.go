package utils

import (
	"testing"

	"github.com/fhsmendes/deploy-cloud-run/models"
)

func TestIsValidCEP(t *testing.T) {
	tests := []struct {
		name     string
		cep      string
		expected bool
	}{
		{"valid CEP 8 digits", "01001000", true},
		{"valid CEP 8 digits", "14406802", true},
		{"valid CEP all zeros", "00000000", true},
		{"valid CEP all nines", "99999999", true},
		{"invalid CEP with dash", "01001-000", false},
		{"invalid CEP with dash different format", "14406-802", false},
		{"invalid CEP too short", "0100100", false},
		{"invalid CEP too short 1 digit", "1", false},
		{"invalid CEP too long", "010010000", false},
		{"invalid CEP too long many digits", "01001000123", false},
		{"invalid CEP with letters", "0100100a", false},
		{"invalid CEP with letters mixed", "01a01000", false},
		{"invalid CEP all letters", "abcdefgh", false},
		{"empty CEP", "", false},
		{"invalid CEP with spaces", "01001 000", false},
		{"invalid CEP with spaces at start", " 01001000", false},
		{"invalid CEP with spaces at end", "01001000 ", false},
		{"invalid CEP with special chars", "01001@00", false},
		{"invalid CEP with dots", "01.001.000", false},
		{"invalid CEP with parentheses", "(01001000)", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidCEP(tt.cep)
			if result != tt.expected {
				t.Errorf("IsValidCEP(%q) = %v, want %v", tt.cep, result, tt.expected)
			}
		})
	}
}

func TestConvertTemperatures(t *testing.T) {
	tests := []struct {
		name      string
		celsius   float64
		expectedC float64
		expectedF float64
		expectedK float64
	}{
		{
			name:      "zero celsius",
			celsius:   0,
			expectedC: 0,
			expectedF: 32,
			expectedK: 273,
		},
		{
			name:      "freezing point",
			celsius:   0,
			expectedC: 0,
			expectedF: 32,
			expectedK: 273,
		},
		{
			name:      "room temperature",
			celsius:   25,
			expectedC: 25,
			expectedF: 77,
			expectedK: 298,
		},
		{
			name:      "body temperature",
			celsius:   37,
			expectedC: 37,
			expectedF: 98.6,
			expectedK: 310,
		},
		{
			name:      "hot summer day",
			celsius:   40,
			expectedC: 40,
			expectedF: 104,
			expectedK: 313,
		},
		{
			name:      "negative temperature",
			celsius:   -10,
			expectedC: -10,
			expectedF: 14,
			expectedK: 263,
		},
		{
			name:      "very cold temperature",
			celsius:   -40,
			expectedC: -40,
			expectedF: -40,
			expectedK: 233,
		},
		{
			name:      "absolute zero celsius",
			celsius:   -273,
			expectedC: -273,
			expectedF: -459.4,
			expectedK: 0,
		},
		{
			name:      "decimal temperature",
			celsius:   23.5,
			expectedC: 23.5,
			expectedF: 74.3,
			expectedK: 296.5,
		},
		{
			name:      "negative decimal temperature",
			celsius:   -12.8,
			expectedC: -12.8,
			expectedF: 8.96,
			expectedK: 260.2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertTemperatures(tt.celsius)

			// Check Celsius
			if result.TempC != tt.expectedC {
				t.Errorf("ConvertTemperatures(%f).TempC = %f, want %f",
					tt.celsius, result.TempC, tt.expectedC)
			}

			// Check Fahrenheit (with small tolerance for floating point precision)
			if !almostEqual(result.TempF, tt.expectedF, 0.01) {
				t.Errorf("ConvertTemperatures(%f).TempF = %f, want %f",
					tt.celsius, result.TempF, tt.expectedF)
			}

			// Check Kelvin
			if !almostEqual(result.TempK, tt.expectedK, 0.01) {
				t.Errorf("ConvertTemperatures(%f).TempK = %f, want %f",
					tt.celsius, result.TempK, tt.expectedK)
			}
		})
	}
}

// Helper function to compare floating point numbers with tolerance
func almostEqual(a, b, tolerance float64) bool {
	if a > b {
		return a-b <= tolerance
	}
	return b-a <= tolerance
}

func TestConvertTemperatures_StructType(t *testing.T) {
	result := ConvertTemperatures(25.0)

	// Verify the returned type is models.Temperature
	if _, ok := interface{}(result).(models.Temperature); !ok {
		t.Error("ConvertTemperatures should return models.Temperature type")
	}

	// Verify all fields are populated
	if result.TempC == 0 && result.TempF == 0 && result.TempK == 0 {
		t.Error("ConvertTemperatures should populate all temperature fields")
	}
}

// Test edge cases separately
func TestIsValidCEP_EdgeCases(t *testing.T) {
	edgeCases := []struct {
		name string
		cep  string
		want bool
	}{
		{"nil-like empty", "", false},
		{"single char", "1", false},
		{"seven chars", "1234567", false},
		{"nine chars", "123456789", false},
		{"unicode digits", "０１００１０００", false}, // Full-width digits
		{"hex-like", "abcd1234", false},
		{"mixed case", "Abcd1234", false},
	}

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsValidCEP(tc.cep); got != tc.want {
				t.Errorf("IsValidCEP(%q) = %v, want %v", tc.cep, got, tc.want)
			}
		})
	}
}

func TestConvertTemperatures_EdgeCases(t *testing.T) {
	edgeCases := []struct {
		name    string
		celsius float64
	}{
		{"very large positive", 1000.0},
		{"very large negative", -1000.0},
		{"smallest positive", 0.1},
		{"smallest negative", -0.1},
	}

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ConvertTemperatures(tc.celsius)

			// Basic sanity checks
			expectedF := tc.celsius*1.8 + 32
			expectedK := tc.celsius + 273

			if !almostEqual(result.TempF, expectedF, 0.01) {
				t.Errorf("Fahrenheit conversion failed for %f: got %f, want %f",
					tc.celsius, result.TempF, expectedF)
			}

			if !almostEqual(result.TempK, expectedK, 0.01) {
				t.Errorf("Kelvin conversion failed for %f: got %f, want %f",
					tc.celsius, result.TempK, expectedK)
			}
		})
	}
}
