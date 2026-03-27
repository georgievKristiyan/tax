package nra

import (
	"encoding/xml"
	"fmt"

	"github.com/tax-cli/internal/utils"
)

// Appendix5 represents Appendix 5 of the tax declaration (sold stocks/securities).
type Appendix5 struct {
	XMLName    xml.Name    `xml:"app5"`
	Table1     *Table1     `xml:"table1"`
	T1Row7     float64     `xml:"t1row7"`
	Table2     *Table2     `xml:"table2"`
	T2Row6Pr   float64     `xml:"t2row6pr"`
	T2Row6Ls   float64     `xml:"t2row6ls"`
	T2Row7     float64     `xml:"t2row7"`
	Part2Row1  float64     `xml:"part2row1"`
	Part2Row2  *string     `xml:"part2row2,omitempty"`
	Part2Row3  *string     `xml:"part2row3,omitempty"`
	Part2Row4  float64     `xml:"part2row4"`
	PartStatus int         `xml:"partstatus"`
}

// Table1 represents table 1 in Appendix 5.
type Table1 struct {
	XMLName xml.Name       `xml:"table1"`
	Rows    []Table1RowEnum `xml:"rowenum"`
}

// Table1RowEnum represents a row in table 1.
type Table1RowEnum struct {
	Code         *int     `xml:"code,omitempty"`
	SellValue    *float64 `xml:"sellvalue,omitempty"`
	BuyValue     *float64 `xml:"buyvalue,omitempty"`
	PosDiff      *float64 `xml:"posdiff,omitempty"`
	Expense      *float64 `xml:"expense,omitempty"`
	PartValue    *float64 `xml:"partvalue,omitempty"`
	LeasingValue *float64 `xml:"leasingvalue,omitempty"`
	Value        *float64 `xml:"value,omitempty"`
}

// Table2 represents table 2 in Appendix 5.
type Table2 struct {
	XMLName xml.Name      `xml:"table2"`
	Rows    []TableTwoRow `xml:"rowenum"`
}

// TableTwoRow represents a row in table 2 of Appendix 5.
type TableTwoRow struct {
	XMLName   xml.Name `xml:"rowenum"`
	Code      int      `xml:"code"`
	SellValue float64  `xml:"sellvalue"`
	BuyValue  float64  `xml:"buyvalue"`
	Profit    float64  `xml:"profit"`
	Loss      float64  `xml:"loss"`
}

// NewTableTwoRow creates a new TableTwoRow with the given values.
func NewTableTwoRow(code int, sellValue, buyValue, profit, loss float64) TableTwoRow {
	return TableTwoRow{
		Code:      code,
		SellValue: utils.RoundToTwoDecimals(sellValue),
		BuyValue:  utils.RoundToTwoDecimals(buyValue),
		Profit:    utils.RoundToTwoDecimals(profit),
		Loss:      utils.RoundToTwoDecimals(loss),
	}
}

// Accumulate combines two TableTwoRow instances.
func (t TableTwoRow) Accumulate(other TableTwoRow) TableTwoRow {
	return NewTableTwoRow(
		t.Code,
		t.SellValue+other.SellValue,
		t.BuyValue+other.BuyValue,
		t.Profit+other.Profit,
		t.Loss+other.Loss,
	)
}

// String returns a formatted string representation of the row.
func (t TableTwoRow) String() string {
	return fmt.Sprintf("Code: %d\tPrice sold: %.2f\tPrice acquired: %.2f\tProfit: %.2f\tLoss: %.2f",
		t.Code, t.SellValue, t.BuyValue, t.Profit, t.Loss)
}

// NewAppendix5 creates a new Appendix5 from a list of TableTwoRow entries.
func NewAppendix5(rows []TableTwoRow) *Appendix5 {
	if len(rows) == 0 {
		return nil
	}

	// Calculate totals
	var totalProfit, totalLoss float64
	for _, row := range rows {
		totalProfit += row.Profit
		totalLoss += row.Loss
	}

	totalProfit = utils.RoundToTwoDecimals(totalProfit)
	totalLoss = utils.RoundToTwoDecimals(totalLoss)

	profit := totalProfit - totalLoss
	t2Row7 := 0.0
	if profit > 0 {
		t2Row7 = utils.RoundToTwoDecimals(profit)
	}

	return &Appendix5{
		Table1: &Table1{
			Rows: []Table1RowEnum{{}},
		},
		T1Row7: 0,
		Table2: &Table2{
			Rows: rows,
		},
		T2Row6Pr:   totalProfit,
		T2Row6Ls:   totalLoss,
		T2Row7:     t2Row7,
		Part2Row1:  t2Row7,
		Part2Row4:  t2Row7,
		PartStatus: 1,
	}
}

// String returns a formatted string representation of Appendix 5.
func (a *Appendix5) String() string {
	if a == nil || a.Table2 == nil {
		return "Appendix 5: (empty)"
	}

	result := "Appendix 5:\n"
	for _, row := range a.Table2.Rows {
		result += row.String() + "\n"
	}
	result += fmt.Sprintf("Total Profit: %.2f\nTotal Loss: %.2f\nNet: %.2f",
		a.T2Row6Pr, a.T2Row6Ls, a.T2Row7)
	return result
}

