package nra

import (
	"encoding/xml"
	"fmt"
	"time"

	"github.com/tax-cli/internal/utils"
)

// Appendix8 represents Appendix 8 of the tax declaration (foreign assets/holdings).
type Appendix8 struct {
	XMLName     xml.Name      `xml:"app8"`
	Stocks      *StocksList   `xml:"stocks"`
	JoinStocks  *JoinStocks   `xml:"joinstocks"`
	Place       *Place        `xml:"place"`
	Base        *Place        `xml:"base"`
	Prop        *PropList     `xml:"prop"`
	Part3       *App8Part3    `xml:"part3"`
	Row4        float64       `xml:"row4"`
	Part38al1   *App8Part38al `xml:"part38al1"`
	Sum81al1    float64       `xml:"sum81al1"`
	Part38al58  *App8Part38al `xml:"part38al58"`
	Sum81al58   float64       `xml:"sum81al58"`
	Part38al13  *App8Part38al `xml:"part38al13"`
	Sum38al13   float64       `xml:"sum38al13"`
	PartStatus  int           `xml:"partstatus"`
}

// StocksList wraps the list of stock entries.
type StocksList struct {
	XMLName xml.Name     `xml:"stocks"`
	Stocks  []StocksEnum `xml:"stocksenum"`
}

// StocksEnum represents a stock holding entry.
type StocksEnum struct {
	XMLName         xml.Name `xml:"stocksenum"`
	Country         string   `xml:"country"`
	Count           float64  `xml:"count"`
	AcquireDate     string   `xml:"acquiredate"` // Format: 2006-01-02
	PriceInCurrency float64  `xml:"priceincurrency"`
	Price           float64  `xml:"price"`
}

// NewStocksEnum creates a new StocksEnum entry.
func NewStocksEnum(country string, count float64, acquireDate time.Time, priceInCurrency, price float64) StocksEnum {
	return StocksEnum{
		Country:         country,
		Count:           utils.RoundToTwoDecimals(count),
		AcquireDate:     acquireDate.Format("2006-01-02"),
		PriceInCurrency: utils.RoundToTwoDecimals(priceInCurrency),
		Price:           utils.RoundToTwoDecimals(price),
	}
}

// String returns a formatted string representation of the stock entry.
func (s StocksEnum) String() string {
	return fmt.Sprintf("Country: %s\tAmount: %.2f\tDate: %s\tIn USD: %.2f\tIn BGN: %.2f",
		s.Country, s.Count, s.AcquireDate, s.PriceInCurrency, s.Price)
}

// JoinStocks represents joined stock entries.
type JoinStocks struct {
	XMLName xml.Name     `xml:"joinstocks"`
	Stocks  []StocksEnum `xml:"stocksenum"`
}

// Place represents a location entry.
type Place struct {
	Country *string `xml:"country,omitempty"`
	Address *string `xml:"addres,omitempty"` // Note: typo matches original XML schema
}

// PropList wraps property entries.
type PropList struct {
	XMLName xml.Name   `xml:"prop"`
	Props   []PropEnum `xml:"propenum"`
}

// PropEnum represents a property entry.
type PropEnum struct {
	Country     *string `xml:"country,omitempty"`
	Address     *string `xml:"addres,omitempty"`
	Type        *string `xml:"type,omitempty"`
	AcquireDate *string `xml:"acquiredate,omitempty"`
}

// App8Part3 represents part 3 of Appendix 8.
type App8Part3 struct {
	XMLName xml.Name         `xml:"part3"`
	Rows    []App8Part3RowEnum `xml:"rowenum"`
}

// App8Part3RowEnum represents a row in part 3.
type App8Part3RowEnum struct {
	Name    *string `xml:"name,omitempty"`
	Country *string `xml:"country,omitempty"`
	OweTax  *string `xml:"owetax,omitempty"`
}

// App8Part38al represents parts 38al1, 38al58, and 38al13.
type App8Part38al struct {
	Rows []App8RowEnum `xml:"rowenum"`
}

// App8RowEnum represents a generic row in Appendix 8 parts.
type App8RowEnum struct {
	Name        *string `xml:"name,omitempty"`
	Country     *string `xml:"country,omitempty"`
	IncomeCode  *string `xml:"incomecode,omitempty"`
	MethodCode  *string `xml:"methodcode,omitempty"`
	Sum         *string `xml:"sum,omitempty"`
	Value       *string `xml:"value,omitempty"`
	Diff        *string `xml:"diff,omitempty"`
	PaidTax     *string `xml:"paidtax,omitempty"`
	PermitedTax *string `xml:"permitedtax,omitempty"`
	Tax         *string `xml:"tax,omitempty"`
	OweTax      *string `xml:"owetax,omitempty"`
}

// NewAppendix8 creates a new Appendix8 from a list of StocksEnum entries.
func NewAppendix8(stocks []StocksEnum) *Appendix8 {
	if len(stocks) == 0 {
		return nil
	}

	return &Appendix8{
		Stocks: &StocksList{
			Stocks: stocks,
		},
		JoinStocks: &JoinStocks{
			Stocks: []StocksEnum{{}}, // Empty placeholder
		},
		Place: &Place{},
		Base:  &Place{},
		Prop: &PropList{
			Props: []PropEnum{{}},
		},
		Part3: &App8Part3{
			Rows: []App8Part3RowEnum{{}},
		},
		Row4: 0,
		Part38al1: &App8Part38al{
			Rows: []App8RowEnum{{}},
		},
		Sum81al1: 0,
		Part38al58: &App8Part38al{
			Rows: []App8RowEnum{{}},
		},
		Sum81al58: 0,
		Part38al13: &App8Part38al{
			Rows: []App8RowEnum{{}},
		},
		Sum38al13:  0,
		PartStatus: 1,
	}
}

// String returns a formatted string representation of Appendix 8.
func (a *Appendix8) String() string {
	if a == nil || a.Stocks == nil {
		return "Appendix 8: (empty)"
	}

	result := "Appendix 8:\n"
	for _, stock := range a.Stocks.Stocks {
		result += stock.String() + "\n"
	}
	return result
}

