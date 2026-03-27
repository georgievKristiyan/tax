// Package cmd provides the CLI commands for the tax declaration tool.
package cmd

import (
	"encoding/xml"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tax-cli/internal/bnb"
	"github.com/tax-cli/internal/broker"
	"github.com/tax-cli/internal/nra"
	"github.com/xuri/excelize/v2"
)

var (
	// Broker-specific input files
	ibkrFile        string
	schwabHoldings  string
	schwabSold      string
	schwabDividends string

	// Output files
	outputFile     string
	appendix8Excel string

	// Verbose flag
	verbose bool
)

// DividendRow represents a processed dividend for display
type DividendRow struct {
	Symbol      string
	Country     string
	Date        string
	AmountBGN   float64 // amount * exchange rate
	TaxWithheld float64 // amount * 0.10 * exchange rate
	TaxCredited float64 // amount * 0.05 * exchange rate
	TaxOwed     float64 // always 0
}

// debugLog prints a message only if verbose mode is enabled
func debugLog(format string, args ...interface{}) {
	if verbose {
		fmt.Printf("[DEBUG] "+format+"\n", args...)
	}
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "tax-cli",
	Short: "Bulgarian tax declaration generator for stock holdings and sales",
	Long: `A CLI tool to generate Bulgarian NRA tax declaration appendices (5 and 8) 
from stock broker export files.

Supports multiple brokers simultaneously:
  - IBKR:   Interactive Brokers (single Flex Query XML)
  - Schwab: Separate holdings, sold, and transaction history (dividends) files

The tool reads broker export file(s), converts USD values to BGN using 
Bulgarian National Bank exchange rates, and generates an XML file that 
can be imported into the NRA tax declaration system.

Examples:
  # IBKR only
  tax-cli --ibkr flex_query.xml

  # Schwab only (holdings + sold + dividends)
  tax-cli --schwab-holdings positions.csv --schwab-sold gains.csv --schwab-dividends transactions.csv

  # Multiple brokers combined
  tax-cli --ibkr flex_query.xml --schwab-sold gains.csv
  
  # With verbose output
  tax-cli --ibkr flex_query.xml --verbose`,
	RunE: runGenerate,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// IBKR - single file
	rootCmd.Flags().StringVar(&ibkrFile, "ibkr", "",
		"IBKR Flex Query XML file (contains both holdings and trades)")

	// Schwab - separate files
	rootCmd.Flags().StringVar(&schwabHoldings, "schwab-holdings", "",
		"Schwab holdings/positions file (CSV)")
	rootCmd.Flags().StringVar(&schwabSold, "schwab-sold", "",
		"Schwab gains/losses file (CSV)")
	rootCmd.Flags().StringVar(&schwabDividends, "schwab-dividends", "",
		"Schwab transaction history file for dividends (CSV)")

	// Output
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "",
		"Output XML file path (optional, if not specified no file is created)")
	rootCmd.Flags().StringVar(&appendix8Excel, "appendix8-excel", "",
		"Output Excel file path for Appendix 8 foreign holdings (for NRA web import)")

	// Verbose
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false,
		"Enable verbose output with detailed calculation logs")
}

func runGenerate(cmd *cobra.Command, args []string) error {
	// Check if at least one input is provided
	if ibkrFile == "" && schwabHoldings == "" && schwabSold == "" && schwabDividends == "" {
		return fmt.Errorf("at least one broker input file must be specified")
	}

	debugLog("Verbose mode enabled")

	// Initialize rate retriever with persistent cache
	rateRetriever := bnb.NewRateRetriever()
	defer func() {
		if err := rateRetriever.SaveCache(); err != nil {
			fmt.Printf("Warning: failed to save rate cache: %v\n", err)
		}
	}()

	// Collect all sold stocks, holdings, and dividends from all brokers
	var allSoldStocks []broker.SoldStock
	var allHoldings []broker.HoldingStock
	var allDividends []broker.Dividend

	// Process IBKR
	if ibkrFile != "" {
		fmt.Printf("Processing IBKR: %s\n", ibkrFile)
		parser := &broker.IBKRParser{}

		debugLog("Parsing IBKR sold stocks from %s", ibkrFile)
		soldStocks, err := parser.ParseSoldStocks(ibkrFile)
		if err != nil {
			return fmt.Errorf("failed to parse IBKR sold stocks: %w", err)
		}
		allSoldStocks = append(allSoldStocks, soldStocks...)
		fmt.Printf("  Found %d sold stock transactions\n", len(soldStocks))

		if verbose {
			for i, ss := range soldStocks {
				debugLog("  Sold #%d: Acquired=%s, Sold=%s, CostBasis=%.2f USD, Proceeds=%.2f USD",
					i+1, ss.DateAcquired.Format("2006-01-02"), ss.DateSold.Format("2006-01-02"),
					ss.AdjustedCostBasis, ss.TotalProceeds)
			}
		}

		debugLog("Parsing IBKR holdings from %s", ibkrFile)
		holdings, err := parser.ParseHoldings(ibkrFile)
		if err != nil {
			return fmt.Errorf("failed to parse IBKR holdings: %w", err)
		}
		allHoldings = append(allHoldings, holdings...)
		fmt.Printf("  Found %d holdings\n", len(holdings))

		if verbose {
			for i, h := range holdings {
				debugLog("  Holding #%d: Date=%s, Qty=%.2f, Price=%.2f USD",
					i+1, h.Date.Format("2006-01-02"), h.Amount, h.Price)
			}
		}

		debugLog("Parsing IBKR dividends from %s", ibkrFile)
		dividends, err := parser.ParseDividends(ibkrFile)
		if err != nil {
			return fmt.Errorf("failed to parse IBKR dividends: %w", err)
		}
		allDividends = append(allDividends, dividends...)
		fmt.Printf("  Found %d dividend payments\n", len(dividends))

		if verbose {
			for i, d := range dividends {
				debugLog("  Dividend #%d: Symbol=%s, Date=%s, Amount=%.2f %s, Country=%s",
					i+1, d.Symbol, d.Date.Format("2006-01-02"), d.Amount, d.Currency, d.IssuerCountryCode)
			}
		}
	}

	// Process Schwab dividends (transaction history CSV)
	if schwabDividends != "" {
		fmt.Printf("Processing Schwab dividends: %s\n", schwabDividends)
		debugLog("Parsing Schwab dividends from %s", schwabDividends)
		schwabParser := &broker.SchwabParser{}
		dividends, err := schwabParser.ParseDividends(schwabDividends)
		if err != nil {
			return fmt.Errorf("failed to parse Schwab dividends: %w", err)
		}
		allDividends = append(allDividends, dividends...)
		fmt.Printf("  Found %d dividend payments\n", len(dividends))

		if verbose {
			for i, d := range dividends {
				debugLog("  Dividend #%d: Symbol=%s, Date=%s, Amount=%.2f %s, Country=%s",
					i+1, d.Symbol, d.Date.Format("2006-01-02"), d.Amount, d.Currency, d.IssuerCountryCode)
			}
		}
	}

	// Process Schwab
	if schwabHoldings != "" || schwabSold != "" {
		parser := broker.GetParser(broker.BrokerSchwab)

		if schwabSold != "" {
			fmt.Printf("Processing Schwab sold: %s\n", schwabSold)
			debugLog("Parsing Schwab sold stocks from %s", schwabSold)
			soldStocks, err := parser.ParseSoldStocks(schwabSold)
			if err != nil {
				return fmt.Errorf("failed to parse Schwab sold stocks: %w", err)
			}
			allSoldStocks = append(allSoldStocks, soldStocks...)
			fmt.Printf("  Found %d sold stock transactions\n", len(soldStocks))

			if verbose {
				for i, ss := range soldStocks {
					debugLog("  Sold #%d: Acquired=%s, Sold=%s, CostBasis=%.2f USD, Proceeds=%.2f USD",
						i+1, ss.DateAcquired.Format("2006-01-02"), ss.DateSold.Format("2006-01-02"),
						ss.AdjustedCostBasis, ss.TotalProceeds)
				}
			}
		}

		if schwabHoldings != "" {
			fmt.Printf("Processing Schwab holdings: %s\n", schwabHoldings)
			debugLog("Parsing Schwab holdings from %s", schwabHoldings)
			holdings, err := parser.ParseHoldings(schwabHoldings)
			if err != nil {
				return fmt.Errorf("failed to parse Schwab holdings: %w", err)
			}
			allHoldings = append(allHoldings, holdings...)
			fmt.Printf("  Found %d holdings\n", len(holdings))

			if verbose {
				for i, h := range holdings {
					debugLog("  Holding #%d: Date=%s, Qty=%.2f, Price=%.2f USD",
						i+1, h.Date.Format("2006-01-02"), h.Amount, h.Price)
				}
			}
		}
	}

	fmt.Println()

	// Process combined data
	var appendix5 *nra.Appendix5
	var appendix8 *nra.Appendix8
	var dividendRows []DividendRow

	if len(allSoldStocks) > 0 {
		fmt.Printf("Processing %d total sold stock transactions...\n", len(allSoldStocks))
		appendix5 = processAppendix5(rateRetriever, allSoldStocks)
	}

	if len(allHoldings) > 0 {
		fmt.Printf("Processing %d total holdings...\n", len(allHoldings))
		appendix8 = processAppendix8(rateRetriever, allHoldings)
	}

	if len(allDividends) > 0 {
		fmt.Printf("Processing %d total dividend payments...\n", len(allDividends))
		dividendRows = processDividends(rateRetriever, allDividends)
	}

	// Write XML output only if -o flag is specified
	if outputFile != "" {
		debugLog("Creating NRA declaration XML")
		declaration := nra.NewDeclaration(appendix5, appendix8)

		output, err := xml.MarshalIndent(declaration, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal XML: %w", err)
		}

		xmlContent := []byte(xml.Header + string(output))

		debugLog("Writing XML to %s (%d bytes)", outputFile, len(xmlContent))
		if err := os.WriteFile(outputFile, xmlContent, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}

		fmt.Printf("Declaration written to: %s\n", outputFile)
	}

	// Write Appendix 8 Excel file if requested
	if appendix8Excel != "" && appendix8 != nil {
		debugLog("Writing Appendix 8 Excel to %s", appendix8Excel)
		if err := writeAppendix8Excel(appendix8, appendix8Excel); err != nil {
			return fmt.Errorf("failed to write Appendix 8 Excel: %w", err)
		}
		fmt.Printf("Appendix 8 Excel written to: %s\n", appendix8Excel)
	}

	// Print summary tables at the end
	fmt.Println()
	fmt.Println(strings.Repeat("=", 100))
	fmt.Println("SUMMARY")
	fmt.Println(strings.Repeat("=", 100))

	if appendix5 != nil {
		printAppendix5Table(appendix5)
	}

	if appendix8 != nil {
		printAppendix8Table(appendix8)
	}

	if len(dividendRows) > 0 {
		printDividendsTable(dividendRows)
	}

	fmt.Println(strings.Repeat("=", 100))

	return nil
}

func printAppendix5Table(app5 *nra.Appendix5) {
	fmt.Println()
	fmt.Println("APPENDIX 5 - Sold Securities (Приложение 5)")
	fmt.Println(strings.Repeat("-", 100))

	if app5.Table2 == nil || len(app5.Table2.Rows) == 0 {
		fmt.Println("No data")
		return
	}

	// Header
	fmt.Printf("%-8s %18s %18s %18s %18s\n",
		"Code", "Sell Value (BGN)", "Buy Value (BGN)", "Profit (BGN)", "Loss (BGN)")
	fmt.Println(strings.Repeat("-", 100))

	// Data rows
	for _, row := range app5.Table2.Rows {
		fmt.Printf("%-8d %18.2f %18.2f %18.2f %18.2f\n",
			row.Code, row.SellValue, row.BuyValue, row.Profit, row.Loss)
	}

	fmt.Println(strings.Repeat("-", 100))

	// Totals
	fmt.Printf("%-8s %18.2f %18s %18.2f %18.2f\n",
		"TOTAL", app5.T2Row6Pr-app5.T2Row6Ls, "", app5.T2Row6Pr, app5.T2Row6Ls)
	fmt.Printf("%-8s %18.2f\n", "NET", app5.T2Row7)
	fmt.Println()
}

func printAppendix8Table(app8 *nra.Appendix8) {
	fmt.Println()
	fmt.Println("APPENDIX 8 - Foreign Holdings (Приложение 8)")
	fmt.Println(strings.Repeat("-", 100))

	if app8.Stocks == nil || len(app8.Stocks.Stocks) == 0 {
		fmt.Println("No data")
		return
	}

	// Header
	fmt.Printf("%-10s %12s %12s %18s %18s\n",
		"Country", "Quantity", "Date", "Value (USD)", "Value (BGN)")
	fmt.Println(strings.Repeat("-", 100))

	// Data rows
	var totalUSD, totalBGN float64
	for _, stock := range app8.Stocks.Stocks {
		fmt.Printf("%-10s %12.2f %12s %18.2f %18.2f\n",
			stock.Country, stock.Count, stock.AcquireDate, stock.PriceInCurrency, stock.Price)
		totalUSD += stock.PriceInCurrency
		totalBGN += stock.Price
	}

	fmt.Println(strings.Repeat("-", 100))

	// Totals
	fmt.Printf("%-10s %12s %12s %18.2f %18.2f\n",
		"TOTAL", "", "", totalUSD, totalBGN)
	fmt.Println()
}

func printDividendsTable(dividends []DividendRow) {
	fmt.Println()
	fmt.Println("APPENDIX 8 Part 3 - Dividends (Приложение 8, Част 3)")
	fmt.Println(strings.Repeat("-", 100))

	// Header
	fmt.Printf("%-8s %-10s %12s %15s %15s %15s %12s\n",
		"Symbol", "Country", "Date", "Amount (BGN)", "Tax Paid 10%", "Tax Credit 5%", "Tax Owed")
	fmt.Println(strings.Repeat("-", 100))

	// Data rows
	var totalAmount, totalTaxWithheld, totalTaxCredited, totalTaxOwed float64
	for _, div := range dividends {
		fmt.Printf("%-8s %-10s %12s %15.2f %15.2f %15.2f %12.2f\n",
			div.Symbol, div.Country, div.Date, div.AmountBGN, div.TaxWithheld, div.TaxCredited, div.TaxOwed)
		totalAmount += div.AmountBGN
		totalTaxWithheld += div.TaxWithheld
		totalTaxCredited += div.TaxCredited
		totalTaxOwed += div.TaxOwed
	}

	fmt.Println(strings.Repeat("-", 100))

	// Totals
	fmt.Printf("%-8s %-10s %12s %15.2f %15.2f %15.2f %12.2f\n",
		"TOTAL", "", "", totalAmount, totalTaxWithheld, totalTaxCredited, totalTaxOwed)
	fmt.Println()
}

func processAppendix5(rr *bnb.RateRetriever, soldStocks []broker.SoldStock) *nra.Appendix5 {
	if len(soldStocks) == 0 {
		return nil
	}

	debugLog("Processing Appendix 5 - Sold Securities")
	debugLog("  Aggregating %d transactions into code 508", len(soldStocks))

	// Aggregate all transactions into a single row with code 508
	var totalBuyValue, totalSellValue, totalProfit, totalLoss float64

	for i, ss := range soldStocks {
		rateAcquired, err := rr.RetrieveRate(ss.DateAcquired)
		if err != nil {
			fmt.Printf("  Warning: failed to get rate for %s: %v\n", ss.DateAcquired.Format("2006-01-02"), err)
			continue
		}

		rateSold, err := rr.RetrieveRate(ss.DateSold)
		if err != nil {
			fmt.Printf("  Warning: failed to get rate for %s: %v\n", ss.DateSold.Format("2006-01-02"), err)
			continue
		}

		buyValue := ss.AdjustedCostBasis * rateAcquired
		sellValue := ss.TotalProceeds * rateSold
		difference := sellValue - buyValue

		debugLog("  Transaction #%d:", i+1)
		debugLog("    Acquired: %s, Cost: %.2f USD × %.4f = %.2f BGN",
			ss.DateAcquired.Format("2006-01-02"), ss.AdjustedCostBasis, rateAcquired, buyValue)
		debugLog("    Sold: %s, Proceeds: %.2f USD × %.4f = %.2f BGN",
			ss.DateSold.Format("2006-01-02"), ss.TotalProceeds, rateSold, sellValue)
		debugLog("    Difference: %.2f BGN (%s)", difference, func() string {
			if difference > 0 {
				return "PROFIT"
			} else if difference < 0 {
				return "LOSS"
			}
			return "BREAK-EVEN"
		}())

		totalBuyValue += buyValue
		totalSellValue += sellValue

		if difference > 0 {
			totalProfit += difference
		} else {
			totalLoss += difference
		}
	}

	debugLog("  Totals: BuyValue=%.2f BGN, SellValue=%.2f BGN, Profit=%.2f BGN, Loss=%.2f BGN",
		totalBuyValue, totalSellValue, totalProfit, totalLoss)

	row := nra.NewTableTwoRow(508, totalSellValue, totalBuyValue, totalProfit, totalLoss)
	return nra.NewAppendix5([]nra.TableTwoRow{row})
}

func processAppendix8(rr *bnb.RateRetriever, holdings []broker.HoldingStock) *nra.Appendix8 {
	if len(holdings) == 0 {
		return nil
	}

	debugLog("Processing Appendix 8 - Foreign Holdings")
	debugLog("  Processing %d holdings", len(holdings))

	// Sort holdings by date (oldest first)
	sort.Slice(holdings, func(i, j int) bool {
		return holdings[i].Date.Before(holdings[j].Date)
	})

	var stocks []nra.StocksEnum
	var totalUSD, totalBGN float64

	for i, h := range holdings {
		rate, err := rr.RetrieveRate(h.Date)
		if err != nil {
			fmt.Printf("  Warning: failed to get rate for %s: %v\n", h.Date.Format("2006-01-02"), err)
			continue
		}

		priceInCurrency := h.Price * h.Amount
		priceInBGN := priceInCurrency * rate

		debugLog("  Holding #%d:", i+1)
		debugLog("    Date: %s, Qty: %.2f, Price/Share: %.2f USD",
			h.Date.Format("2006-01-02"), h.Amount, h.Price)
		debugLog("    Total USD: %.2f × %.2f = %.2f USD",
			h.Price, h.Amount, priceInCurrency)
		debugLog("    Rate: %.4f, Total BGN: %.2f × %.4f = %.2f BGN",
			rate, priceInCurrency, rate, priceInBGN)

		totalUSD += priceInCurrency
		totalBGN += priceInBGN

		stock := nra.NewStocksEnum(
			"САЩ", // USA in Bulgarian
			h.Amount,
			h.Date,
			priceInCurrency,
			priceInBGN,
		)
		stocks = append(stocks, stock)
	}

	debugLog("  Totals: %.2f USD = %.2f BGN", totalUSD, totalBGN)

	return nra.NewAppendix8(stocks)
}

func processDividends(rr *bnb.RateRetriever, dividends []broker.Dividend) []DividendRow {
	if len(dividends) == 0 {
		return nil
	}

	debugLog("Processing Appendix 8 Part 3 - Dividends")
	debugLog("  Processing %d dividend payments", len(dividends))

	// Sort dividends by date
	sort.Slice(dividends, func(i, j int) bool {
		return dividends[i].Date.Before(dividends[j].Date)
	})

	var rows []DividendRow
	var totalAmountUSD, totalAmountBGN float64

	for i, div := range dividends {
		rate, err := rr.RetrieveRate(div.Date)
		if err != nil {
			fmt.Printf("  Warning: failed to get rate for %s: %v\n", div.Date.Format("2006-01-02"), err)
			continue
		}

		amountBGN := div.Amount * rate
		taxWithheld := div.Amount * 0.10 * rate // 10% tax withheld
		taxCredited := div.Amount * 0.05 * rate // 5% tax credit allowed

		debugLog("  Dividend #%d (%s):", i+1, div.Symbol)
		debugLog("    Date: %s, Amount: %.2f %s",
			div.Date.Format("2006-01-02"), div.Amount, div.Currency)
		debugLog("    Rate: %.4f, Amount BGN: %.2f × %.4f = %.2f BGN",
			rate, div.Amount, rate, amountBGN)
		debugLog("    Tax Withheld (10%%): %.2f × 0.10 × %.4f = %.2f BGN",
			div.Amount, rate, taxWithheld)
		debugLog("    Tax Credit (5%%): %.2f × 0.05 × %.4f = %.2f BGN",
			div.Amount, rate, taxCredited)

		totalAmountUSD += div.Amount
		totalAmountBGN += amountBGN

		rows = append(rows, DividendRow{
			Symbol:      div.Symbol,
			Country:     div.IssuerCountryCode,
			Date:        div.Date.Format("2006-01-02"),
			AmountBGN:   amountBGN,
			TaxWithheld: taxWithheld,
			TaxCredited: taxCredited,
			TaxOwed:     0, // Always 0
		})
	}

	debugLog("  Totals: %.2f USD = %.2f BGN", totalAmountUSD, totalAmountBGN)

	return rows
}

// writeAppendix8Excel writes the Appendix 8 foreign holdings to an Excel file matching
// the exact format expected by the NRA portal:
//   - Row 1: numeric headers 1–6, all formatted as text (numFmtId=49)
//   - Data rows: col A = "Акции" (bold Arial 12pt), col B = country (text),
//     col C = quantity (2 decimals), col D = acquire date (date serial),
//     col E = USD value (2 decimals), col F = BGN value (2 decimals)
func writeAppendix8Excel(app8 *nra.Appendix8, filePath string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"

	// --- Styles ---
	// Style: text format (numFmtId=49, @)
	textStyle, err := f.NewStyle(&excelize.Style{
		NumFmt: 49,
	})
	if err != nil {
		return err
	}

	// Style: date format (numFmtId=14, MM/DD/YYYY)
	dateStyle, err := f.NewStyle(&excelize.Style{
		NumFmt: 14,
	})
	if err != nil {
		return err
	}

	// Style: 2-decimal number (numFmtId=2, 0.00)
	numStyle, err := f.NewStyle(&excelize.Style{
		NumFmt: 2,
	})
	if err != nil {
		return err
	}

	// Style: bold Arial 12pt dark (for col A "Акции")
	boldStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:   true,
			Size:   12,
			Family: "Arial",
			Color:  "495668",
		},
	})
	if err != nil {
		return err
	}

	// --- Header row (row 1): values 1–6, text style ---
	for col := 1; col <= 6; col++ {
		cell, _ := excelize.CoordinatesToCellName(col, 1)
		if err := f.SetCellValue(sheet, cell, col); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, cell, cell, textStyle); err != nil {
			return err
		}
	}

	// --- Data rows ---
	for i, s := range app8.Stocks.Stocks {
		row := i + 2

		// Col A: "Акции" with bold style
		cellA, _ := excelize.CoordinatesToCellName(1, row)
		if err := f.SetCellStr(sheet, cellA, "Акции"); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, cellA, cellA, boldStyle); err != nil {
			return err
		}

		// Col B: country, text style
		cellB, _ := excelize.CoordinatesToCellName(2, row)
		if err := f.SetCellStr(sheet, cellB, s.Country); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, cellB, cellB, textStyle); err != nil {
			return err
		}

		// Col C: quantity, 2-decimal style
		cellC, _ := excelize.CoordinatesToCellName(3, row)
		if err := f.SetCellFloat(sheet, cellC, s.Count, 2, 64); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, cellC, cellC, numStyle); err != nil {
			return err
		}

		// Col D: acquire date as Excel serial number, date style
		acquireDate, err := time.Parse("2006-01-02", s.AcquireDate)
		if err != nil {
			return fmt.Errorf("invalid acquire date %q: %w", s.AcquireDate, err)
		}
		cellD, _ := excelize.CoordinatesToCellName(4, row)
		if err := f.SetCellValue(sheet, cellD, acquireDate); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, cellD, cellD, dateStyle); err != nil {
			return err
		}

		// Col E: USD value, 2-decimal style
		cellE, _ := excelize.CoordinatesToCellName(5, row)
		if err := f.SetCellFloat(sheet, cellE, s.PriceInCurrency, 2, 64); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, cellE, cellE, numStyle); err != nil {
			return err
		}

		// Col F: BGN value, 2-decimal style
		cellF, _ := excelize.CoordinatesToCellName(6, row)
		if err := f.SetCellFloat(sheet, cellF, s.Price, 2, 64); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, cellF, cellF, numStyle); err != nil {
			return err
		}
	}

	return f.SaveAs(filePath)
}
