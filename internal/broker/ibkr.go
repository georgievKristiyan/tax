package broker

import (
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// IBKRParser implements the Parser interface for Interactive Brokers (IBKR).
// It parses IBKR Flex Query XML exports.
type IBKRParser struct{}

// Name returns the broker name.
func (p *IBKRParser) Name() string {
	return "Interactive Brokers (IBKR)"
}

// FlexQueryResponse is the root element of IBKR Flex Query XML.
type FlexQueryResponse struct {
	XMLName        xml.Name       `xml:"FlexQueryResponse"`
	QueryName      string         `xml:"queryName,attr"`
	Type           string         `xml:"type,attr"`
	FlexStatements FlexStatements `xml:"FlexStatements"`
}

// FlexStatements contains the list of flex statements.
type FlexStatements struct {
	Count      int             `xml:"count,attr"`
	Statements []FlexStatement `xml:"FlexStatement"`
}

// FlexStatement represents a single account statement.
type FlexStatement struct {
	AccountID        string           `xml:"accountId,attr"`
	FromDate         string           `xml:"fromDate,attr"`
	ToDate           string           `xml:"toDate,attr"`
	WhenGenerated    string           `xml:"whenGenerated,attr"`
	OpenPositions    OpenPositions    `xml:"OpenPositions"`
	Trades           Trades           `xml:"Trades"`
	TradeConfirms    TradeConfirms    `xml:"TradeConfirms"`
	CashTransactions CashTransactions `xml:"CashTransactions"`
}

// OpenPositions contains the list of open positions.
type OpenPositions struct {
	Positions []OpenPosition `xml:"OpenPosition"`
}

// OpenPosition represents a single open position (holding).
type OpenPosition struct {
	Position              string `xml:"position,attr"`
	CostBasisMoney        string `xml:"costBasisMoney,attr"`
	OpenDateTime          string `xml:"openDateTime,attr"`
	Currency              string `xml:"currency,attr"`
	AssetCategory         string `xml:"assetCategory,attr"`
	Symbol                string `xml:"symbol,attr"`
	Description           string `xml:"description,attr"`
	IssuerCountryCode     string `xml:"issuerCountryCode,attr"`
	CostBasisPrice        string `xml:"costBasisPrice,attr"`
	HoldingPeriodDateTime string `xml:"holdingPeriodDateTime,attr"`
}

// Trades contains the list of trades (closed lots from AF type query).
type Trades struct {
	Lots []TradeLot `xml:"Lot"`
}

// TradeLot represents a closed lot (sold stock) from AF type Flex Query.
// Contains cost basis and acquisition info.
type TradeLot struct {
	Currency              string `xml:"currency,attr"`
	AssetCategory         string `xml:"assetCategory,attr"`
	Symbol                string `xml:"symbol,attr"`
	Description           string `xml:"description,attr"`
	IssuerCountryCode     string `xml:"issuerCountryCode,attr"`
	DateTime              string `xml:"dateTime,attr"`
	TradeDate             string `xml:"tradeDate,attr"`
	Quantity              string `xml:"quantity,attr"`
	TradePrice            string `xml:"tradePrice,attr"`
	Proceeds              string `xml:"proceeds,attr"`
	Cost                  string `xml:"cost,attr"`
	FifoPnlRealized       string `xml:"fifoPnlRealized,attr"`
	OpenDateTime          string `xml:"openDateTime,attr"`
	HoldingPeriodDateTime string `xml:"holdingPeriodDateTime,attr"`
	BuySell               string `xml:"buySell,attr"`
	LevelOfDetail         string `xml:"levelOfDetail,attr"`
}

// TradeConfirms contains trade confirmations (from TCF type query).
type TradeConfirms struct {
	Confirms        []TradeConfirm  `xml:"TradeConfirm"`
	Orders          []TradeOrder    `xml:"Order"`
	SymbolSummaries []SymbolSummary `xml:"SymbolSummary"`
}

// TradeConfirm represents a single trade execution.
type TradeConfirm struct {
	Currency      string `xml:"currency,attr"`
	AssetCategory string `xml:"assetCategory,attr"`
	Symbol        string `xml:"symbol,attr"`
	Description   string `xml:"description,attr"`
	DateTime      string `xml:"dateTime,attr"`
	TradeDate     string `xml:"tradeDate,attr"`
	Quantity      string `xml:"quantity,attr"`
	Price         string `xml:"price,attr"`
	Amount        string `xml:"amount,attr"`
	Proceeds      string `xml:"proceeds,attr"`
	NetCash       string `xml:"netCash,attr"`
	BuySell       string `xml:"buySell,attr"`
	LevelOfDetail string `xml:"levelOfDetail,attr"`
	OrderID       string `xml:"orderID,attr"`
}

// TradeOrder represents an order-level summary.
type TradeOrder struct {
	Currency      string `xml:"currency,attr"`
	AssetCategory string `xml:"assetCategory,attr"`
	Symbol        string `xml:"symbol,attr"`
	DateTime      string `xml:"dateTime,attr"`
	TradeDate     string `xml:"tradeDate,attr"`
	Quantity      string `xml:"quantity,attr"`
	Price         string `xml:"price,attr"`
	Amount        string `xml:"amount,attr"`
	Proceeds      string `xml:"proceeds,attr"`
	NetCash       string `xml:"netCash,attr"`
	BuySell       string `xml:"buySell,attr"`
	LevelOfDetail string `xml:"levelOfDetail,attr"`
	OrderID       string `xml:"orderID,attr"`
}

// SymbolSummary represents a symbol-level summary.
type SymbolSummary struct {
	Currency      string `xml:"currency,attr"`
	AssetCategory string `xml:"assetCategory,attr"`
	Symbol        string `xml:"symbol,attr"`
	TradeDate     string `xml:"tradeDate,attr"`
	Quantity      string `xml:"quantity,attr"`
	Price         string `xml:"price,attr"`
	Amount        string `xml:"amount,attr"`
	Proceeds      string `xml:"proceeds,attr"`
	BuySell       string `xml:"buySell,attr"`
	LevelOfDetail string `xml:"levelOfDetail,attr"`
}

// CashTransactions contains cash transactions including dividends.
type CashTransactions struct {
	Transactions []CashTransaction `xml:"CashTransaction"`
}

// CashTransaction represents a cash transaction (dividend, interest, etc.).
type CashTransaction struct {
	Currency          string `xml:"currency,attr"`
	AssetCategory     string `xml:"assetCategory,attr"`
	Symbol            string `xml:"symbol,attr"`
	Description       string `xml:"description,attr"`
	IssuerCountryCode string `xml:"issuerCountryCode,attr"`
	DateTime          string `xml:"dateTime,attr"`
	SettleDate        string `xml:"settleDate,attr"`
	Amount            string `xml:"amount,attr"`
	Type              string `xml:"type,attr"`
	LevelOfDetail     string `xml:"levelOfDetail,attr"`
	SubCategory       string `xml:"subCategory,attr"`
	DividendType      string `xml:"dividendType,attr"`
}

// ParseSoldStocks parses IBKR Flex Query XML for sold stocks.
//
// The AF type query (TaxInfo) contains Trades/Lot with:
// - cost: adjusted cost basis
// - openDateTime: acquisition date
// - dateTime/tradeDate: sale date
// - quantity: number of shares
// - fifoPnlRealized: realized profit/loss (proceeds - cost)
//
// We can calculate proceeds as: cost + fifoPnlRealized
func (p *IBKRParser) ParseSoldStocks(filePath string) ([]SoldStock, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var response FlexQueryResponse
	if err := xml.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	var soldStocks []SoldStock

	for _, stmt := range response.FlexStatements.Statements {
		// Parse from Trades/Lot (AF type - has complete info)
		for _, lot := range stmt.Trades.Lots {
			// Only process closed lots (sells)
			if lot.LevelOfDetail != "CLOSED_LOT" {
				continue
			}

			// Only process stock sales
			if lot.AssetCategory != "STK" {
				continue
			}

			// Parse acquisition date
			dateAcquired, err := parseIBKRFlexDate(lot.OpenDateTime)
			if err != nil {
				dateAcquired, err = parseIBKRFlexDate(lot.HoldingPeriodDateTime)
				if err != nil {
					fmt.Printf("  Warning: could not parse acquisition date for %s: %v\n", lot.Symbol, err)
					continue
				}
			}

			// Parse sale date
			dateSold, err := parseIBKRFlexDate(lot.DateTime)
			if err != nil {
				dateSold, err = parseIBKRFlexDate(lot.TradeDate)
				if err != nil {
					fmt.Printf("  Warning: could not parse sale date for %s: %v\n", lot.Symbol, err)
					continue
				}
			}

			// Parse cost basis
			cost, err := strconv.ParseFloat(lot.Cost, 64)
			if err != nil {
				fmt.Printf("  Warning: could not parse cost for %s: %v\n", lot.Symbol, err)
				continue
			}

			// Calculate proceeds from cost + fifoPnlRealized
			// fifoPnlRealized = proceeds - cost, so proceeds = cost + fifoPnlRealized
			proceeds := cost // default to cost if we can't calculate
			if lot.FifoPnlRealized != "" {
				if pnl, err := strconv.ParseFloat(lot.FifoPnlRealized, 64); err == nil {
					proceeds = cost + pnl
				}
			}

			soldStocks = append(soldStocks, SoldStock{
				DateAcquired:      dateAcquired,
				AdjustedCostBasis: cost,
				DateSold:          dateSold,
				TotalProceeds:     proceeds,
			})
		}
	}

	return soldStocks, nil
}

// ParseHoldings parses IBKR Flex Query XML for open positions (holdings).
// Uses the OpenPositions/OpenPosition elements.
func (p *IBKRParser) ParseHoldings(filePath string) ([]HoldingStock, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var response FlexQueryResponse
	if err := xml.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	var holdings []HoldingStock
	currentYear := time.Now().Year()

	for _, stmt := range response.FlexStatements.Statements {
		for _, pos := range stmt.OpenPositions.Positions {
			// Only process stocks
			if pos.AssetCategory != "STK" {
				continue
			}

			// Parse date
			date, err := parseIBKRFlexDate(pos.OpenDateTime)
			if err != nil {
				date, err = parseIBKRFlexDate(pos.HoldingPeriodDateTime)
				if err != nil {
					fmt.Printf("  Warning: could not parse date for %s: %v\n", pos.Symbol, err)
					continue
				}
			}

			// Filter stocks acquired in current year (only include previous years)
			if date.Year() >= currentYear {
				continue
			}

			// Parse position (quantity)
			amount, err := strconv.ParseFloat(pos.Position, 64)
			if err != nil {
				fmt.Printf("  Warning: could not parse position for %s: %v\n", pos.Symbol, err)
				continue
			}

			// Parse cost basis price (per share)
			price, err := strconv.ParseFloat(pos.CostBasisPrice, 64)
			if err != nil {
				// Try to calculate from total cost basis and position
				if costBasis, err := strconv.ParseFloat(pos.CostBasisMoney, 64); err == nil && amount != 0 {
					price = costBasis / amount
				} else {
					fmt.Printf("  Warning: could not parse price for %s: %v\n", pos.Symbol, err)
					continue
				}
			}

			holdings = append(holdings, HoldingStock{
				Date:     date,
				Amount:   amount,
				Price:    price,
				Currency: pos.Currency,
				Country:  pos.IssuerCountryCode,
			})
		}
	}

	return holdings, nil
}

// ParseDividends parses IBKR Flex Query XML for dividend payments.
// Uses the CashTransactions/CashTransaction elements with type="Dividends".
func (p *IBKRParser) ParseDividends(filePath string) ([]Dividend, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var response FlexQueryResponse
	if err := xml.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	var dividends []Dividend

	for _, stmt := range response.FlexStatements.Statements {
		for _, tx := range stmt.CashTransactions.Transactions {
			// Only process dividends
			if tx.Type != "Dividends" {
				continue
			}

			// Only process detail level (not summaries)
			if tx.LevelOfDetail != "DETAIL" {
				continue
			}

			// Parse date
			date, err := parseIBKRFlexDate(tx.DateTime)
			if err != nil {
				date, err = parseIBKRFlexDate(tx.SettleDate)
				if err != nil {
					fmt.Printf("  Warning: could not parse date for dividend %s: %v\n", tx.Symbol, err)
					continue
				}
			}

			// Parse amount
			amount, err := strconv.ParseFloat(tx.Amount, 64)
			if err != nil {
				fmt.Printf("  Warning: could not parse amount for dividend %s: %v\n", tx.Symbol, err)
				continue
			}

			dividends = append(dividends, Dividend{
				Symbol:            tx.Symbol,
				Date:              date,
				Amount:            amount,
				Currency:          tx.Currency,
				IssuerCountryCode: tx.IssuerCountryCode,
				SubCategory:       tx.SubCategory,
				DividendType:      tx.DividendType,
			})
		}
	}

	return dividends, nil
}

// parseIBKRFlexDate parses IBKR Flex Query date formats.
// Formats: "DD/MM/YYYY", "DD/MM/YYYY HH:MM:SS"
func parseIBKRFlexDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}

	// IBKR Flex Query uses DD/MM/YYYY format
	formats := []string{
		"02/01/2006 15:04:05", // DD/MM/YYYY HH:MM:SS
		"02/01/2006",          // DD/MM/YYYY
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", s)
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
