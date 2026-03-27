// Package broker provides interfaces and implementations for parsing stock data from various brokers.
package broker

import (
	"time"
)

// SoldStock represents a stock that has been sold.
type SoldStock struct {
	DateAcquired      time.Time
	AdjustedCostBasis float64
	DateSold          time.Time
	TotalProceeds     float64
}

// HoldingStock represents a stock holding (sellable stock).
type HoldingStock struct {
	Date   time.Time
	Amount float64
	Price  float64
}

// Dividend represents a dividend payment received.
type Dividend struct {
	Symbol            string
	Date              time.Time
	Amount            float64 // Gross dividend amount in original currency
	Currency          string
	IssuerCountryCode string
}

// Parser defines the interface for parsing broker-specific data files.
type Parser interface {
	// ParseSoldStocks parses a file containing sold stock transactions.
	ParseSoldStocks(filePath string) ([]SoldStock, error)

	// ParseHoldings parses a file containing stock holdings.
	ParseHoldings(filePath string) ([]HoldingStock, error)

	// Name returns the broker name.
	Name() string
}

// DividendParser is an optional interface for parsers that support dividend parsing.
type DividendParser interface {
	// ParseDividends parses a file containing dividend payments.
	ParseDividends(filePath string) ([]Dividend, error)
}

// BrokerType represents supported broker types.
type BrokerType string

const (
	BrokerIBKR   BrokerType = "ibkr"
	BrokerSchwab BrokerType = "schwab"
)

// GetParser returns the appropriate parser for the given broker type.
func GetParser(brokerType BrokerType) Parser {
	switch brokerType {
	case BrokerIBKR:
		return &IBKRParser{}
	case BrokerSchwab:
		return &SchwabParser{}
	default:
		return nil
	}
}
