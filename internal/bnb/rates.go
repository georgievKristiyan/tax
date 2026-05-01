// Package bnb provides functionality to retrieve exchange rates from the Bulgarian National Bank (BNB).
package bnb

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"strconv"
	"sync"
	"time"
)

const (
	cacheFileName = ".bnb_rates_cache.json"
)

// RateRetriever retrieves and caches exchange rates from BNB.
type RateRetriever struct {
	client    *http.Client
	cache     map[string]float64 // key: date string, value: rate
	mu        sync.RWMutex
	cacheFile string
	dirty     bool // tracks if cache has been modified
}

// NewRateRetriever creates a new RateRetriever instance with persistent cache.
func NewRateRetriever() *RateRetriever {
	rr := &RateRetriever{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache: make(map[string]float64),
	}

	// Determine cache file path in home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Warning: could not get home directory, using in-memory cache only: %v\n", err)
		return rr
	}

	rr.cacheFile = filepath.Join(homeDir, cacheFileName)
	rr.loadCache()

	return rr
}

// loadCache loads the cache from the persistent file.
func (rr *RateRetriever) loadCache() {
	if rr.cacheFile == "" {
		return
	}

	data, err := os.ReadFile(rr.cacheFile)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("Warning: could not read cache file: %v\n", err)
		}
		return
	}

	rr.mu.Lock()
	defer rr.mu.Unlock()

	if err := json.Unmarshal(data, &rr.cache); err != nil {
		fmt.Printf("Warning: could not parse cache file: %v\n", err)
		rr.cache = make(map[string]float64)
	}

	fmt.Printf("Loaded %d cached exchange rates from %s\n", len(rr.cache), rr.cacheFile)
}

// SaveCache saves the cache to the persistent file.
// Call this before the program exits to persist new rates.
func (rr *RateRetriever) SaveCache() error {
	if rr.cacheFile == "" {
		return nil
	}

	rr.mu.RLock()
	defer rr.mu.RUnlock()

	if !rr.dirty {
		return nil // No changes to save
	}

	data, err := json.MarshalIndent(rr.cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	if err := os.WriteFile(rr.cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	fmt.Printf("Saved %d exchange rates to cache\n", len(rr.cache))
	return nil
}

// Rates represents the XML response from BNB.
type Rates struct {
	XMLName xml.Name `xml:"ROWSET"`
	Rows    []Rate   `xml:"ROW"`
}

// Rate represents a single currency rate entry.
type Rate struct {
	Code string `xml:"CODE"`
	Rate string `xml:"RATE"`
}

// GetRate returns the exchange rate for the given currency code.
func (r *Rates) GetRate(code string) (float64, error) {
	for _, rate := range r.Rows {
		if rate.Code == code {
			return strconv.ParseFloat(rate.Rate, 64)
		}
	}
	return 0, fmt.Errorf("rate not found for currency: %s", code)
}

// RetrieveRate retrieves the USD to BGN exchange rate for a given date.
// It caches results to avoid repeated API calls.
func (rr *RateRetriever) RetrieveRate(date time.Time) (float64, error) {
	dateKey := date.Format("2006-01-02")

	// Check cache first
	rr.mu.RLock()
	if rate, ok := rr.cache[dateKey]; ok {
		rr.mu.RUnlock()
		return rate, nil
	}
	rr.mu.RUnlock()

	// Fetch from BNB
	rate, err := rr.fetchRateFromBNB(date)
	if err != nil {
		return 0, err
	}

	// Cache the result
	rr.mu.Lock()
	rr.cache[dateKey] = rate
	rr.dirty = true
	rr.mu.Unlock()

	return rate, nil
}

// fetchRateFromBNB fetches the exchange rate from BNB API.
func (rr *RateRetriever) fetchRateFromBNB(date time.Time) (float64, error) {
	u, err := url.Parse("https://www.bnb.bg/Statistics/StExternalSector/StExchangeRates/StERForeignCurrencies/index.htm")
	if err != nil {
		return 0, err
	}

	q := u.Query()
	q.Set("downloadOper", "true")
	q.Set("group1", "first")
	q.Set("firstDays", strconv.Itoa(date.Day()))
	q.Set("firstMonths", date.Month().String())
	q.Set("firstYear", strconv.Itoa(date.Year()))
	q.Set("search", "true")
	q.Set("showChart", "false")
	q.Set("showChartButton", "false")
	q.Set("type", "XML")
	u.RawQuery = q.Encode()

	resp, err := rr.client.Get(u.String())
	if err != nil {
		return 0, fmt.Errorf("failed to fetch rate: %w", err)
	}
	defer resp.Body.Close()

	// Check if we got XML response (rate exists for this date)
	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/xml;charset=UTF-8" && contentType != "text/xml; charset=UTF-8" {
		// No rate for this date (weekend/holiday), try previous day
		return rr.fetchRateFromBNB(date.AddDate(0, 0, -1))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response: %w", err)
	}

	var rates Rates
	if err := xml.Unmarshal(body, &rates); err != nil {
		return 0, fmt.Errorf("failed to parse XML: %w", err)
	}

	return rates.GetRate("USD")
}

// RetrieveRateForCurrency retrieves the exchange rate for a specific currency.
func (rr *RateRetriever) RetrieveRateForCurrency(date time.Time, currency string) (float64, error) {
	// EUR to BGN is a fixed peg in Bulgaria
	if strings.ToUpper(currency) == "EUR" {
		return 1.95583, nil
	}

	dateKey := date.Format("2006-01-02") + "_" + currency

	// Check cache first
	rr.mu.RLock()
	if rate, ok := rr.cache[dateKey]; ok {
		rr.mu.RUnlock()
		return rate, nil
	}
	rr.mu.RUnlock()

	// Fetch from BNB
	rate, err := rr.fetchRateForCurrencyFromBNB(date, currency)
	if err != nil {
		return 0, err
	}

	// Cache the result
	rr.mu.Lock()
	rr.cache[dateKey] = rate
	rr.dirty = true
	rr.mu.Unlock()

	return rate, nil
}

func (rr *RateRetriever) fetchRateForCurrencyFromBNB(date time.Time, currency string) (float64, error) {
	u, err := url.Parse("https://www.bnb.bg/Statistics/StExternalSector/StExchangeRates/StERForeignCurrencies/index.htm")
	if err != nil {
		return 0, err
	}

	q := u.Query()
	q.Set("downloadOper", "true")
	q.Set("group1", "first")
	q.Set("firstDays", strconv.Itoa(date.Day()))
	q.Set("firstMonths", date.Month().String())
	q.Set("firstYear", strconv.Itoa(date.Year()))
	q.Set("search", "true")
	q.Set("showChart", "false")
	q.Set("showChartButton", "false")
	q.Set("type", "XML")
	u.RawQuery = q.Encode()

	resp, err := rr.client.Get(u.String())
	if err != nil {
		return 0, fmt.Errorf("failed to fetch rate: %w", err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/xml;charset=UTF-8" && contentType != "text/xml; charset=UTF-8" {
		return rr.fetchRateForCurrencyFromBNB(date.AddDate(0, 0, -1), currency)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response: %w", err)
	}

	var rates Rates
	if err := xml.Unmarshal(body, &rates); err != nil {
		return 0, fmt.Errorf("failed to parse XML: %w", err)
	}

	return rates.GetRate(currency)
}

// CacheStats returns the number of cached rates.
func (rr *RateRetriever) CacheStats() int {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	return len(rr.cache)
}
