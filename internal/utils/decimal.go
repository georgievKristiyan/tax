// Package utils provides utility functions for the tax CLI application.
package utils

import (
	"math"
)

// RoundToTwoDecimals rounds a float64 to two decimal places using half-up rounding.
func RoundToTwoDecimals(value float64) float64 {
	return math.Round(value*100) / 100
}

// FormatCurrency formats a float64 as a currency string with two decimal places.
func FormatCurrency(value float64) string {
	return formatFloat(RoundToTwoDecimals(value))
}

func formatFloat(value float64) string {
	if value == 0 {
		return "0.00"
	}
	// Use fixed precision formatting
	return fixedPrecision(value, 2)
}

func fixedPrecision(value float64, precision int) string {
	format := "%." + string(rune('0'+precision)) + "f"
	return sprintf(format, value)
}

// sprintf is a simple implementation to avoid importing fmt in this utility package
func sprintf(format string, value float64) string {
	// Simple implementation for "%.2f" format
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}

	intPart := int64(value)
	fracPart := int64(math.Round((value - float64(intPart)) * 100))

	if fracPart >= 100 {
		intPart++
		fracPart -= 100
	}

	intStr := intToString(intPart)
	fracStr := intToString(fracPart)

	if len(fracStr) == 1 {
		fracStr = "0" + fracStr
	}

	return sign + intStr + "." + fracStr
}

func intToString(n int64) string {
	if n == 0 {
		return "0"
	}

	result := ""
	for n > 0 {
		digit := n % 10
		result = string(rune('0'+digit)) + result
		n /= 10
	}
	return result
}

