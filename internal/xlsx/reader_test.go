package xlsx

import (
	"math"
	"path/filepath"
	"testing"
)

func TestOpenRealWorkbook(t *testing.T) {
	p := filepath.Join("..", "..", "24.07.05. Первое полугодие 2024 (данные).xlsx")
	wb, err := OpenFile(p)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}
	if len(wb.Sheets) != 16 {
		t.Fatalf("expected 16 sheets, got %d: %v", len(wb.Sheets), wb.SheetNames())
	}

	s1 := wb.Sheet("Остатки всего")
	if s1 == nil {
		t.Fatal("missing sheet Остатки всего")
	}
	if got := s1.Float(5, 2); math.Abs(got-6236485) > 0.01 {
		t.Errorf("B5 expected 6236485, got %v", got)
	}
	if got := s1.Float(23, 4); math.Abs(got-10958068.06) > 0.01 {
		t.Errorf("D23 expected 10958068.06, got %v", got)
	}

	s2 := wb.Sheet("Склады")
	if s2 == nil {
		t.Fatal("missing sheet Склады")
	}
	if got := s2.String(1, 2); got != "31.12.2023" {
		t.Errorf("Склады B1 date expected 31.12.2023, got %q", got)
	}
	if got := s2.String(1, 3); got != "30.06.2024" {
		t.Errorf("Склады C1 date expected 30.06.2024, got %q", got)
	}

	sz := wb.Sheet("Закупки РФ-РФ")
	if sz == nil {
		t.Fatal("missing sheet Закупки РФ-РФ")
	}
	if got := sz.Cell(11, 1).Indent; got != 0 {
		t.Errorf("Row 11 indent expected 0, got %d", got)
	}
	if got := sz.Cell(12, 1).Indent; got != 2 {
		t.Errorf("Row 12 indent expected 2, got %d", got)
	}
	if got := sz.Cell(13, 1).Indent; got != 4 {
		t.Errorf("Row 13 indent expected 4, got %d", got)
	}
	if got := sz.Float(1715, 9); math.Abs(got-68034018.54) > 0.01 {
		t.Errorf("Закупки РФ-РФ Итого expected 68034018.54, got %v", got)
	}
}
