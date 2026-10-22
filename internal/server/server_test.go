package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppServerEndpoints(t *testing.T) {
	xlsxPath := filepath.Join("..", "..", "24.07.05. Первое полугодие 2024 (данные).xlsx")
	raw, err := os.ReadFile(xlsxPath)
	if err != nil {
		t.Fatalf("read xlsx: %v", err)
	}

	srv, err := New(Options{
		DefaultFileName:  "24.07.05. Первое полугодие 2024 (данные).xlsx",
		DefaultFileBytes: raw,
		DesktopMode:      false,
	})
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	h := srv.Handler()

	// 1. GET /api/dashboard
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d", rec.Code)
	}
	var payload struct {
		Report struct {
			FileName   string `json:"fileName"`
			SheetCount int    `json:"sheetCount"`
		} `json:"report"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode dashboard: %v", err)
	}
	if payload.Report.SheetCount != 16 {
		t.Errorf("expected 16 sheets, got %d", payload.Report.SheetCount)
	}

	// 2. POST /api/upload
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "uploaded_report.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(raw)
	_ = mw.Close()

	upReq := httptest.NewRequest(http.MethodPost, "/api/upload", &body)
	upReq.Header.Set("Content-Type", mw.FormDataContentType())
	upRec := httptest.NewRecorder()
	h.ServeHTTP(upRec, upReq)
	if upRec.Code != http.StatusOK {
		t.Fatalf("upload status = %d, body = %s", upRec.Code, upRec.Body.String())
	}
	if !strings.Contains(upRec.Body.String(), "uploaded_report.xlsx") {
		t.Errorf("expected uploaded filename in response")
	}

	// 3. GET /api/export/csv?section=retail
	csvReq := httptest.NewRequest(http.MethodGet, "/api/export/csv?section=retail", nil)
	csvRec := httptest.NewRecorder()
	h.ServeHTTP(csvRec, csvReq)
	if csvRec.Code != http.StatusOK {
		t.Fatalf("csv export status = %d", csvRec.Code)
	}
	csvBytes := csvRec.Body.Bytes()
	if len(csvBytes) < 3 || csvBytes[0] != 0xEF || csvBytes[1] != 0xBB || csvBytes[2] != 0xBF {
		t.Errorf("expected UTF-8 BOM in CSV export")
	}
	if !strings.Contains(string(csvBytes), "Брест") {
		t.Errorf("expected Брест in retail CSV export")
	}
}
