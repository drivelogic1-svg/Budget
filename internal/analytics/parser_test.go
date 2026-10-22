package analytics

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/drivelogic1-svg/Budget/internal/xlsx"
)

func assertNear(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s: got %.2f, want %.2f (diff %.4f)", name, got, want, got-want)
	}
}

func TestAnalyzeRealBudgetWorkbook(t *testing.T) {
	p := filepath.Join("..", "..", "24.07.05. Первое полугодие 2024 (данные).xlsx")
	wb, err := xlsx.OpenFile(p)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}

	rep := AnalyzeWorkbook("24.07.05. Первое полугодие 2024 (данные).xlsx", wb)
	if !rep.IsBudgetDataset {
		t.Fatal("expected IsBudgetDataset=true")
	}

	// Verify Overview KPIs
	assertNear(t, "SalesH2_2023", rep.KPIs.SalesH2_2023, 9512304.48, 0.05)
	assertNear(t, "SalesH1_2024", rep.KPIs.SalesH1_2024, 10958068.06, 0.05)
	assertNear(t, "SalesGrowthPct", rep.KPIs.SalesGrowthPct, 15.20, 0.05)
	assertNear(t, "StockEnd2023", rep.KPIs.StockEnd2023, 5281388.00, 0.05)
	assertNear(t, "StockMid2024", rep.KPIs.StockMid2024, 6236485.00, 0.05)
	assertNear(t, "StockGrowthH1_2024Pct", rep.KPIs.StockGrowthH1_2024Pct, 18.08, 0.05)

	// Verify Karcher
	if len(rep.Karcher.Monthly) != 18 {
		t.Errorf("expected 18 Karcher months, got %d", len(rep.Karcher.Monthly))
	}
	assertNear(t, "Karcher TotalSales18m", rep.Karcher.TotalSales18m, 10841627.29, 0.05)
	assertNear(t, "Karcher TotalPurch18m", rep.Karcher.TotalPurch18m, 11642026.41, 0.05)
	if len(rep.Karcher.Segments2023) != 12 {
		t.Errorf("expected 12 Karcher segment months, got %d", len(rep.Karcher.Segments2023))
	}

	// Verify Retail
	if len(rep.Retail.Stores) != 4 {
		t.Fatalf("expected 4 retail stores, got %d", len(rep.Retail.Stores))
	}
	if len(rep.Retail.Monthly) != 15 {
		t.Errorf("expected 15 retail months, got %d", len(rep.Retail.Monthly))
	}
	assertNear(t, "Retail TotalStock24_03", rep.Retail.TotalStock24_03, 456734.14, 0.05)
	if rep.Retail.Stores[0].City != "Брест" {
		t.Errorf("expected top retail city Брест, got %q", rep.Retail.Stores[0].City)
	}
	assertNear(t, "Brest StockMonths", rep.Retail.Stores[0].StockMonths, 2.16, 0.02)

	// Verify Groups
	if len(rep.Groups.Items) != 20 {
		t.Errorf("expected 20 supplier groups, got %d", len(rep.Groups.Items))
	}
	assertNear(t, "Groups TrackedEnd", rep.Groups.TrackedEnd, 5864894.47, 0.05)

	// Verify SMG -> ChST Intercompany
	assertNear(t, "SMG->ChST TotalQ4_23", rep.Intercompany.TotalQ4_23, 490432.53, 0.05)
	assertNear(t, "SMG->ChST TotalQ1_24", rep.Intercompany.TotalQ1_24, 685636.32, 0.05)
	assertNear(t, "SMG->ChST Top4Share", rep.Intercompany.Top4Share, 80.31, 0.05)

	// Verify Prepayments
	assertNear(t, "Prepayments GrandTotalBYN", rep.Prepayments.GrandTotalBYN, 2749340.58, 0.05)
	assertNear(t, "Prepayments PaidTotalBYN", rep.Prepayments.PaidTotalBYN, 1443044.35, 0.05)
	assertNear(t, "Prepayments UnpaidTotalBYN", rep.Prepayments.UnpaidTotalBYN, 832290.23, 0.05)
	assertNear(t, "Prepayments KarcherRFDebtBYN", rep.Prepayments.KarcherRFDebtBYN, 474006.00, 0.05)

	// Verify RF Procurement
	assertNear(t, "RF ExternalQ4_2023", rep.RFProcurement.ExternalQ4_2023, 9930515.43, 0.05)
	assertNear(t, "RF ExternalQ1_2024", rep.RFProcurement.ExternalQ1_2024, 8949255.64, 0.05)
	assertNear(t, "RF Total1CWithVAT", rep.RFProcurement.Total1CWithVAT, 68034018.54, 0.05)
	if len(rep.RFProcurement.Suppliers1C) != 33 {
		t.Errorf("expected 33 1C suppliers, got %d", len(rep.RFProcurement.Suppliers1C))
	}
	if rep.RFProcurement.LineItemCount < 1200 {
		t.Errorf("expected >1200 1C SKU line items, got %d", rep.RFProcurement.LineItemCount)
	}
}
