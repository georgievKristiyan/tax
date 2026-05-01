package broker

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// parseFloat parses a string that may contain currency symbols and commas into a float64.
func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	// Remove currency symbols and commas
	s = strings.ReplaceAll(s, "$", "")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")

	if s == "" || s == "-" || s == "--" {
		return 0, fmt.Errorf("empty value")
	}

	return strconv.ParseFloat(s, 64)
}

// SchwabParser implements the Parser interface for Charles Schwab broker.
type SchwabParser struct{}

// Name returns the broker name.
func (p *SchwabParser) Name() string {
	return "Charles Schwab"
}

// ParseSoldStocks parses Schwab Gain/Loss report.
// Schwab provides CSV exports with the following structure:
// - Symbol: Stock symbol
// - Quantity: Number of shares sold
// - Date Acquired: Original purchase date
// - Date Sold: Sale date
// - Proceeds: Total sale proceeds
// - Cost Basis: Adjusted cost basis
// - Gain/Loss: Realized gain or loss
//
// Schwab CSV format typically has these columns:
// Symbol, Quantity, Date Acquired, Date Sold, Proceeds, Cost Basis, Short Term Gain/Loss, Long Term Gain/Loss
func (p *SchwabParser) ParseSoldStocks(filePath string) ([]SoldStock, error) {
	if strings.HasSuffix(strings.ToLower(filePath), ".csv") {
		return p.parseSoldStocksCSV(filePath)
	}
	return p.parseSoldStocksExcel(filePath)
}

func (p *SchwabParser) parseSoldStocksCSV(filePath string) ([]SoldStock, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	var soldStocks []SoldStock
	headerIdx := make(map[string]int)
	headerFound := false

	for _, record := range records {
		if len(record) == 0 {
			continue
		}

		// Skip empty or total rows
		if strings.TrimSpace(record[0]) == "" || strings.Contains(strings.ToLower(record[0]), "total") {
			continue
		}

		// Look for header row
		if !headerFound {
			for i, cell := range record {
				cell = strings.TrimSpace(strings.ToLower(cell))
				if cell == "symbol" || cell == "date acquired" || cell == "date sold" ||
					cell == "closed date" || cell == "opened date" {
					headerFound = true
				}
				headerIdx[cell] = i
			}
			if headerFound {
				continue
			}
		}

		if !headerFound {
			continue
		}

		stock, err := p.parseSchwabTradeRecord(record, headerIdx)
		if err != nil {
			continue
		}

		soldStocks = append(soldStocks, stock)
	}

	return soldStocks, nil
}

func (p *SchwabParser) parseSchwabTradeRecord(record []string, headerIdx map[string]int) (SoldStock, error) {
	var stock SoldStock

	// Column name variations (lowercase for matching)
	dateAcquiredCols := []string{"date acquired", "acquired date", "acquisition date", "open date", "opened date"}
	dateSoldCols := []string{"date sold", "sold date", "sale date", "close date", "closed date"}
	costBasisCols := []string{"cost basis (cb)", "cost basis", "adjusted cost basis", "basis", "cost"}
	proceedsCols := []string{"proceeds", "sale proceeds", "gross proceeds", "amount"}

	// Parse date acquired
	for _, col := range dateAcquiredCols {
		if idx, ok := headerIdx[col]; ok && idx < len(record) {
			if t, err := parseSchwabDate(record[idx]); err == nil {
				stock.DateAcquired = t
				break
			}
		}
	}

	// Parse date sold
	for _, col := range dateSoldCols {
		if idx, ok := headerIdx[col]; ok && idx < len(record) {
			if t, err := parseSchwabDate(record[idx]); err == nil {
				stock.DateSold = t
				break
			}
		}
	}

	// Parse cost basis
	for _, col := range costBasisCols {
		if idx, ok := headerIdx[col]; ok && idx < len(record) {
			if v, err := parseFloat(record[idx]); err == nil {
				stock.AdjustedCostBasis = v
				break
			}
		}
	}

	// Parse proceeds
	for _, col := range proceedsCols {
		if idx, ok := headerIdx[col]; ok && idx < len(record) {
			if v, err := parseFloat(record[idx]); err == nil {
				stock.TotalProceeds = v
				break
			}
		}
	}

	if stock.DateSold.IsZero() {
		return stock, fmt.Errorf("missing date sold")
	}

	return stock, nil
}

func parseSchwabDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}

	// Schwab date formats
	formats := []string{
		"01/02/2006",
		"1/2/2006",
		"2006-01-02",
		"Jan 02, 2006",
		"January 02, 2006",
		"02-Jan-2006",
		"2-Jan-2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", s)
}

// parseSchwabTransactionDate handles the transaction history date column which may include
// a settlement note, e.g. "12/18/2025 as of 12/17/2025". It uses the first date segment.
func parseSchwabTransactionDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if idx := strings.Index(strings.ToLower(s), " as of "); idx != -1 {
		s = strings.TrimSpace(s[:idx])
	}
	return parseSchwabDate(s)
}

// dividendActions is the set of Schwab Action values treated as dividend income.
var dividendActions = map[string]bool{
	"qualified dividend":     true,
	"non-qualified dividend": true,
	"cash dividend":          true,
}

// ParseDividends parses a Schwab transaction history CSV export for dividend payments.
// Only rows whose Action field is in dividendActions with a positive Amount are included.
func (p *SchwabParser) ParseDividends(filePath string) ([]Dividend, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	headerIdx := make(map[string]int)
	headerFound := false
	var dividends []Dividend

	for _, record := range records {
		if len(record) == 0 {
			continue
		}

		if !headerFound {
			for i, cell := range record {
				key := strings.TrimSpace(strings.ToLower(cell))
				if key == "date" || key == "action" {
					headerFound = true
				}
				headerIdx[key] = i
			}
			if headerFound {
				continue
			}
		}

		if !headerFound {
			continue
		}

		getField := func(name string) string {
			if idx, ok := headerIdx[name]; ok && idx < len(record) {
				return strings.TrimSpace(record[idx])
			}
			return ""
		}

		action := strings.ToLower(getField("action"))
		if !dividendActions[action] {
			continue
		}

		amount, err := parseFloat(getField("amount"))
		if err != nil || amount <= 0 {
			continue
		}

		date, err := parseSchwabTransactionDate(getField("date"))
		if err != nil {
			fmt.Printf("  Warning: could not parse date for dividend %s: %v\n", getField("symbol"), err)
			continue
		}

		dividends = append(dividends, Dividend{
			Symbol:            getField("symbol"),
			Date:              date,
			Amount:            amount,
			Currency:          "USD",
			IssuerCountryCode: "US",
		})
	}

	return dividends, nil
}

func (p *SchwabParser) parseSoldStocksExcel(filePath string) ([]SoldStock, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get rows: %w", err)
	}

	var soldStocks []SoldStock
	headerIdx := make(map[string]int)
	headerFound := false

	for _, row := range rows {
		if len(row) == 0 {
			continue
		}

		if !headerFound {
			for i, cell := range row {
				cell = strings.TrimSpace(strings.ToLower(cell))
				if cell == "symbol" || cell == "date acquired" || cell == "date sold" {
					headerFound = true
				}
				headerIdx[cell] = i
			}
			if headerFound {
				continue
			}
		}

		if !headerFound {
			continue
		}

		stock, err := p.parseSchwabTradeRecord(row, headerIdx)
		if err != nil {
			continue
		}

		soldStocks = append(soldStocks, stock)
	}

	return soldStocks, nil
}

// ParseHoldings parses Schwab positions/holdings report.
// Schwab positions export typically contains:
// - Symbol: Stock symbol
// - Description: Stock description
// - Quantity: Number of shares
// - Price: Current market price
// - Market Value: Total market value
// - Cost Basis: Total cost basis
// - Gain/Loss: Unrealized gain/loss
//
// For equity awards (like RSUs), the format may include:
// - Grant Date: Original grant date
// - Vest Date: Vesting date (acquisition date for tax purposes)
// - Shares: Number of shares
// - Grant Price: Price at grant
// - Vest Price: Price at vest (cost basis per share)
func (p *SchwabParser) ParseHoldings(filePath string) ([]HoldingStock, error) {
	if strings.HasSuffix(strings.ToLower(filePath), ".csv") {
		return p.parseHoldingsCSV(filePath)
	}
	return p.parseHoldingsExcel(filePath)
}

func (p *SchwabParser) parseHoldingsCSV(filePath string) ([]HoldingStock, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // allow variable column counts
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	var holdings []HoldingStock
	headerIdx := make(map[string]int)
	headerFound := false
	currentYear := time.Now().Year()

	for _, record := range records {
		if len(record) == 0 {
			continue
		}

		// Skip empty or total rows
		if strings.TrimSpace(record[0]) == "" || strings.Contains(strings.ToLower(record[0]), "total") {
			continue
		}

		if !headerFound {
			for i, cell := range record {
				cell = strings.TrimSpace(strings.ToLower(cell))
				if cell == "symbol" || cell == "quantity" || cell == "shares" || cell == "vest date" ||
					strings.HasPrefix(cell, "qty") {
					headerFound = true
				}
				headerIdx[cell] = i
			}
			if headerFound {
				continue
			}
		}

		if !headerFound {
			continue
		}

		holding, err := p.parseSchwabPositionRecord(record, headerIdx)
		if err != nil {
			continue
		}

		// Filter stocks from current year
		if holding.Date.Year() >= currentYear {
			continue
		}

		holdings = append(holdings, holding)
	}

	return holdings, nil
}

func (p *SchwabParser) parseSchwabPositionRecord(record []string, headerIdx map[string]int) (HoldingStock, error) {
	holding := HoldingStock{
		Currency: "USD",
		Country:  "US",
	}

	// Column name variations (lowercase for matching)
	dateCols := []string{"vest date", "acquisition date", "date acquired", "purchase date", "date"}
	quantityCols := []string{"quantity", "shares", "qty", "units", "qty (quantity)"}
	priceCols := []string{"vest price", "cost basis price", "price", "cost per share", "acquisition price"}

	// Parse date
	for _, col := range dateCols {
		if idx, ok := headerIdx[col]; ok && idx < len(record) {
			if t, err := parseSchwabDate(record[idx]); err == nil {
				holding.Date = t
				break
			}
		}
	}

	// Parse quantity
	for _, col := range quantityCols {
		if idx, ok := headerIdx[col]; ok && idx < len(record) {
			if v, err := parseFloat(record[idx]); err == nil {
				holding.Amount = v
				break
			}
		}
	}

	// Parse price
	for _, col := range priceCols {
		if idx, ok := headerIdx[col]; ok && idx < len(record) {
			if v, err := parseFloat(record[idx]); err == nil {
				holding.Price = v
				break
			}
		}
	}

	if holding.Date.IsZero() {
		return holding, fmt.Errorf("missing date")
	}

	return holding, nil
}

func (p *SchwabParser) parseHoldingsExcel(filePath string) ([]HoldingStock, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get rows: %w", err)
	}

	var holdings []HoldingStock
	headerIdx := make(map[string]int)
	headerFound := false
	currentYear := time.Now().Year()

	for _, row := range rows {
		if len(row) == 0 {
			continue
		}

		if !headerFound {
			for i, cell := range row {
				cell = strings.TrimSpace(strings.ToLower(cell))
				if cell == "symbol" || cell == "quantity" || cell == "shares" || cell == "vest date" {
					headerFound = true
				}
				headerIdx[cell] = i
			}
			if headerFound {
				continue
			}
		}

		if !headerFound {
			continue
		}

		holding, err := p.parseSchwabPositionRecord(row, headerIdx)
		if err != nil {
			continue
		}

		if holding.Date.Year() >= currentYear {
			continue
		}

		holdings = append(holdings, holding)
	}

	return holdings, nil
}
