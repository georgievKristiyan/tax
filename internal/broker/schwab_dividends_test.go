package broker

import (
	"testing"
	"time"
)

func TestParseSchwabTransactionDate(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Time
		wantErr bool
	}{
		{"09/15/2025", time.Date(2025, 9, 15, 0, 0, 0, 0, time.UTC), false},
		{"06/15/2025", time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC), false},
		{"12/18/2025 as of 12/17/2025", time.Date(2025, 12, 18, 0, 0, 0, 0, time.UTC), false},
		{"06/10/2025 as of 06/09/2025", time.Date(2025, 6, 10, 0, 0, 0, 0, time.UTC), false},
		{"", time.Time{}, true},
	}

	for _, tt := range tests {
		got, err := parseSchwabTransactionDate(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseSchwabTransactionDate(%q): expected error, got nil", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSchwabTransactionDate(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if !got.Equal(tt.want) {
			t.Errorf("parseSchwabTransactionDate(%q): got %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestSchwabParserParseDividends(t *testing.T) {
	p := &SchwabParser{}
	dividends, err := p.ParseDividends("../../testdata/schwab_transaction_history_dividends.csv")
	if err != nil {
		t.Fatalf("ParseDividends returned error: %v", err)
	}

	// The fixture has 2 Qualified Dividend rows; all other rows must be ignored.
	if len(dividends) != 2 {
		t.Fatalf("expected 2 dividends, got %d", len(dividends))
	}

	tests := []struct {
		symbol string
		date   time.Time
		amount float64
	}{
		{"TEST", time.Date(2025, 9, 15, 0, 0, 0, 0, time.UTC), 85.00},
		{"TEST", time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC), 92.50},
	}

	for i, tt := range tests {
		d := dividends[i]
		if d.Symbol != tt.symbol {
			t.Errorf("dividend[%d].Symbol: got %q, want %q", i, d.Symbol, tt.symbol)
		}
		if !d.Date.Equal(tt.date) {
			t.Errorf("dividend[%d].Date: got %v, want %v", i, d.Date, tt.date)
		}
		if d.Amount != tt.amount {
			t.Errorf("dividend[%d].Amount: got %.2f, want %.2f", i, d.Amount, tt.amount)
		}
		if d.Currency != "USD" {
			t.Errorf("dividend[%d].Currency: got %q, want USD", i, d.Currency)
		}
		if d.IssuerCountryCode != "САЩ" {
			t.Errorf("dividend[%d].IssuerCountryCode: got %q, want САЩ", i, d.IssuerCountryCode)
		}
	}
}

func TestSchwabParserParseDividends_ExcludedActions(t *testing.T) {
	p := &SchwabParser{}
	dividends, err := p.ParseDividends("../../testdata/schwab_transaction_history_dividends.csv")
	if err != nil {
		t.Fatalf("ParseDividends returned error: %v", err)
	}

	for _, d := range dividends {
		if d.Amount < 0 {
			t.Errorf("negative amount in dividend output (NRA Tax Adj leaked?): %+v", d)
		}
	}
}
