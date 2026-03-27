package broker

import (
	"testing"
	"time"
)

func TestSchwabParserParseSoldStocks_RealizedGainLossFormat(t *testing.T) {
	p := &SchwabParser{}
	stocks, err := p.ParseSoldStocks("../../testdata/schwab_gain_loss_realized.csv")
	if err != nil {
		t.Fatalf("ParseSoldStocks returned error: %v", err)
	}

	if len(stocks) != 3 {
		t.Fatalf("expected 3 sold stock records, got %d", len(stocks))
	}

	tests := []struct {
		dateSold     time.Time
		dateAcquired time.Time
		proceeds     float64
		costBasis    float64
	}{
		{time.Date(2025, 11, 20, 0, 0, 0, 0, time.UTC), time.Date(2025, 11, 20, 0, 0, 0, 0, time.UTC), 1202.00, 1240.00},
		{time.Date(2025, 8, 10, 0, 0, 0, 0, time.UTC), time.Date(2025, 8, 10, 0, 0, 0, 0, time.UTC), 862.50, 900.00},
		{time.Date(2025, 5, 5, 0, 0, 0, 0, time.UTC), time.Date(2025, 5, 5, 0, 0, 0, 0, time.UTC), 1560.00, 1500.00},
	}

	for i, tt := range tests {
		s := stocks[i]
		if !s.DateSold.Equal(tt.dateSold) {
			t.Errorf("stocks[%d].DateSold: got %v, want %v", i, s.DateSold, tt.dateSold)
		}
		if !s.DateAcquired.Equal(tt.dateAcquired) {
			t.Errorf("stocks[%d].DateAcquired: got %v, want %v", i, s.DateAcquired, tt.dateAcquired)
		}
		if s.TotalProceeds != tt.proceeds {
			t.Errorf("stocks[%d].TotalProceeds: got %.2f, want %.2f", i, s.TotalProceeds, tt.proceeds)
		}
		if s.AdjustedCostBasis != tt.costBasis {
			t.Errorf("stocks[%d].AdjustedCostBasis: got %.2f, want %.2f", i, s.AdjustedCostBasis, tt.costBasis)
		}
	}
}
