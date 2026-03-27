> [!CAUTION]
> **THIS TOOL IS CREATED TO FIX MY PERSONAL USE CASES AND PROVIDES NO SUPPORT.**

# Bulgarian Tax Declaration CLI

A command-line tool to generate Bulgarian NRA tax declaration appendices (5 and 8) from stock broker export files.

## Features

- **Multi-broker support**: Interactive Brokers (IBKR) and Charles Schwab
- **Automatic currency conversion**: Fetches USD to BGN exchange rates from Bulgarian National Bank (BNB)
- **Persistent rate cache**: Caches exchange rates in `~/.bnb_rates_cache.json` to avoid repeated API calls
- **Dividend processing**: Calculates dividend income with tax withheld and tax credit
- **Modular architecture**: Easy to add support for additional brokers
- **XML output**: Generates XML compatible with NRA's tax declaration import system

## Installation

### Prerequisites

- Go 1.21 or later

### Build from source

```bash
cd go-tax-cli
go mod tidy
go build -o tax-cli .
```

## Usage

```bash
# Show help
./tax-cli --help

# IBKR - single Flex Query XML file
./tax-cli --ibkr /path/to/TaxInfo.xml

# Schwab - separate files for holdings and sold stocks
./tax-cli --schwab-holdings /path/to/positions.csv --schwab-sold /path/to/gains_losses.csv

# Schwab - include dividends from transaction history export
./tax-cli --schwab-holdings /path/to/positions.csv --schwab-sold /path/to/gains_losses.csv --schwab-dividends /path/to/transactions.csv

# Multiple brokers combined
./tax-cli --ibkr /path/to/TaxInfo.xml --schwab-sold /path/to/gains.csv

# Generate XML output file
./tax-cli --ibkr /path/to/TaxInfo.xml -o declaration.xml

# Generate Appendix 8 Excel file for NRA web import
./tax-cli --ibkr /path/to/TaxInfo.xml --appendix8-excel appendix8.xlsx
```

### Command-line Options

| Flag | Short | Description |
|------|-------|-------------|
| `--ibkr` | | IBKR Flex Query XML file (contains holdings, trades, and dividends) |
| `--schwab-holdings` | | Schwab holdings/positions file (CSV) |
| `--schwab-sold` | | Schwab gains/losses file (CSV) |
| `--schwab-dividends` | | Schwab transaction history file for dividends (CSV) |
| `--output` | `-o` | Output XML file path (optional, if not specified no file is created) |
| `--appendix8-excel` | | Output Excel file for Appendix 8 foreign holdings (for NRA web import) |

## Broker-Specific Instructions

### Interactive Brokers (IBKR)

IBKR uses a **single Flex Query XML file** that contains open positions (holdings), closed trades (sold stocks), and cash transactions (dividends).

#### Creating the Flex Query

1. Login to Client Portal
2. Go to **Performance & Reports → Flex Queries**
3. Click **Create** or **+** to create a new Activity Flex Query

**Query Name:** `TaxInfo`

**Sections** (for each section, select all fields):

| Section | Options |
|---|---|
| **Cash Transactions** | Dividends, Detail |
| **Open Positions** | Lot |
| **Trades** | Closed Lots |

**Delivery Configuration:**

| Setting | Value |
|---|---|
| Format | XML |
| Period | Last Month |

**General Configuration:**

| Setting | Value |
|---|---|
| Profit and Loss | Default |
| Include Canceled Trades? | No |
| Include Currency Rates? | No |
| Include Audit Trail Fields? | No |
| Display Account Alias in Place of Account ID? | No |
| Breakout by Day? | No |
| Date Format | `dd/MM/yyyy` |
| Time Format | `HH:mm:ss` |
| Date/Time Separator | ` ` (single space) |

#### How it works

The tool extracts:
- **Holdings (Appendix 8)**: From `OpenPositions/OpenPosition` elements
- **Sold Stocks (Appendix 5)**: From `Trades/Lot` elements where `levelOfDetail="CLOSED_LOT"`
  - Cost basis: `cost` attribute
  - Proceeds: Calculated as `cost + fifoPnlRealized`
  - Acquisition date: `openDateTime` or `holdingPeriodDateTime`
  - Sale date: `dateTime` or `tradeDate`
- **Dividends (Appendix 8 Part 3)**: From `CashTransactions/CashTransaction` elements where `type="Dividends"`
  - Amount in BGN: `amount * exchange rate`
  - Tax withheld (10%): `amount * 0.10 * exchange rate`
  - Tax credit (5%): `amount * 0.05 * exchange rate`

#### Usage

```bash
./tax-cli --ibkr ~/Downloads/TaxInfo.xml
```

### Charles Schwab

Schwab requires **two separate files** (CSV format).

#### Gain/Loss Report (Appendix 5)
1. Login to Schwab
2. Go to **Accounts → Realized Gain/Loss**
3. Filter by tax year
4. Export as CSV

#### Positions Report (Appendix 8)
1. Login to Schwab
2. Go to **Accounts → Positions**
3. Export as CSV

> ⚠️ **Important**: The Positions report only shows **current holdings** at the time of export. Sold positions will not appear. For Appendix 8, you need to report holdings owned at **year-end (December 31st)**. Export positions before selling in the new year, or use year-end account statements.

#### Transaction History — Dividends (Appendix 8 Part 3)

1. Login to Schwab
2. Go to **Accounts → Transaction History**
3. Select the account and set a date range covering the full tax year (e.g. Jan 1 – Dec 31)
4. Click **Export** and choose **CSV**

The exported CSV has the columns: `Date`, `Action`, `Symbol`, `Description`, `Quantity`, `Price`, `Fees & Comm`, `Amount`.

The parser extracts rows where `Action` is `Qualified Dividend` (positive `Amount`) as gross dividend income. Rows with `Action` = `NRA Tax Adj` are the US withholding deductions — these are **not** included as separate entries; instead the tool applies the standard 10%/5% model on the gross amount.

> **Row limit**: Schwab caps exports at ~10,000 rows. If your account has extensive activity, export in smaller date ranges (e.g. per quarter) and run the CLI once per file, or concatenate the CSVs (keeping only one header row) before running.

#### Usage

```bash
./tax-cli --schwab-holdings positions.csv --schwab-sold gains_losses.csv --schwab-dividends transactions.csv
```

## Output

The tool displays summary tables for:

- **Appendix 5** (if sold stocks found): Capital gains/losses from stock sales
- **Appendix 8** (if holdings found): Foreign asset declarations
- **Appendix 8 Part 3** (if dividends found): Dividend income with tax calculations

Example output:
```
====================================================================================================
SUMMARY
====================================================================================================

APPENDIX 5 - Sold Securities (Приложение 5)
----------------------------------------------------------------------------------------------------
Code       Sell Value (BGN)    Buy Value (BGN)       Profit (BGN)         Loss (BGN)
----------------------------------------------------------------------------------------------------
508                43965.13           28524.26           15440.87               0.00
----------------------------------------------------------------------------------------------------
TOTAL              15440.87                              15440.87               0.00
NET                15440.87

APPENDIX 8 Part 3 - Dividends (Приложение 8, Част 3)
----------------------------------------------------------------------------------------------------
Symbol   Country            Date    Amount (BGN)    Tax Paid 10%   Tax Credit 5%     Tax Owed
----------------------------------------------------------------------------------------------------
AVGO     САЩ          2025-03-31         1071.25          107.12           53.56         0.00
----------------------------------------------------------------------------------------------------
TOTAL                                    1071.25          107.12           53.56         0.00

====================================================================================================
```

If `-o` flag is provided, an XML file is also generated for import into NRA.

### Importing to NRA

1. Go to [NRA's e-services portal](https://nra.bg)
2. Start a new annual tax declaration (Годишна данъчна декларация по чл. 50)
3. Use the import function to load the generated XML
4. Review and complete the remaining sections manually

## Exchange Rate Cache

The tool caches BNB exchange rates in `~/.bnb_rates_cache.json` to avoid repeated API calls. The cache persists between runs, so subsequent executions are much faster.

To clear the cache, simply delete the file:
```bash
rm ~/.bnb_rates_cache.json
```

## Architecture

```
go-tax-cli/
├── cmd/
│   └── root.go           # CLI command definitions
├── internal/
│   ├── broker/
│   │   ├── types.go      # Common types and interfaces
│   │   ├── ibkr.go       # IBKR parser (Flex Query XML)
│   │   └── schwab.go     # Schwab parser
│   ├── bnb/
│   │   └── rates.go      # BNB exchange rate retrieval with persistent cache
│   ├── nra/
│   │   ├── declaration.go # Main declaration structure
│   │   ├── appendix5.go   # Appendix 5 (sold stocks)
│   │   └── appendix8.go   # Appendix 8 (holdings)
│   └── utils/
│       └── decimal.go     # Utility functions
├── main.go
├── go.mod
└── README.md
```

## Adding a New Broker

1. Create a new file in `internal/broker/` (e.g., `newbroker.go`)
2. Implement the `Parser` interface:

```go
type Parser interface {
    ParseSoldStocks(filePath string) ([]SoldStock, error)
    ParseHoldings(filePath string) ([]HoldingStock, error)
    Name() string
}
```

3. Optionally implement `DividendParser` for dividend support:

```go
type DividendParser interface {
    ParseDividends(filePath string) ([]Dividend, error)
}
```

4. Add the new broker type to `types.go`:

```go
const (
    BrokerIBKR      BrokerType = "ibkr"
    BrokerSchwab    BrokerType = "schwab"
    BrokerNewBroker BrokerType = "newbroker"  // Add this
)

func GetParser(brokerType BrokerType) Parser {
    switch brokerType {
    // ... existing cases ...
    case BrokerNewBroker:
        return &NewBrokerParser{}
    }
}
```

5. Update `cmd/root.go` to add CLI flags for the new broker

## Disclaimer

⚠️ **Use at your own risk.** This tool is provided as-is without warranty. Always verify the generated data before submitting to NRA. The authors are not responsible for any errors in tax declarations.

## License

MIT License
