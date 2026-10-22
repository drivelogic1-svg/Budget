package server

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/drivelogic1-svg/Budget/internal/analytics"
	"github.com/drivelogic1-svg/Budget/internal/xlsx"
)

// NativeFilePickerFunc opens a native OS file dialog and returns the selected path (or "" if cancelled).
type NativeFilePickerFunc func() (string, error)

// Options configures the Analytics HTTP/GUI server.
type Options struct {
	WebFS            fs.FS
	DefaultFileName  string
	DefaultFileBytes []byte
	InitialFilePath  string
	DesktopMode      bool
	NativePicker     NativeFilePickerFunc
	OnShutdown       func()
}

// AppServer serves the HTML GUI and analytical REST API.
type AppServer struct {
	opts        Options
	mu          sync.RWMutex
	currentWB   *xlsx.Workbook
	currentRep  *analytics.DashboardReport
	defaultWB   *xlsx.Workbook
	defaultName string

	lifeMu        sync.Mutex
	hasConnected  bool
	lastPing      time.Time
	windowClosing bool
}

// New creates and initializes an AppServer, loading the initial or embedded Excel workbook.
func New(opts Options) (*AppServer, error) {
	s := &AppServer{
		opts:        opts,
		defaultName: opts.DefaultFileName,
	}

	if len(opts.DefaultFileBytes) > 0 {
		wb, err := xlsx.OpenBytes(opts.DefaultFileBytes)
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения встроенного Excel: %w", err)
		}
		s.defaultWB = wb
		s.currentWB = wb
		s.currentRep = analytics.AnalyzeWorkbook(opts.DefaultFileName, wb)
	}

	if opts.InitialFilePath != "" {
		if err := s.LoadFileFromDisk(opts.InitialFilePath); err != nil && s.currentRep == nil {
			return nil, err
		}
	}

	if s.currentRep == nil {
		return nil, fmt.Errorf("нет доступного файла данных Excel (.xlsx)")
	}

	if opts.DesktopMode && opts.OnShutdown != nil {
		go s.monitorDesktopLifecycle()
	}

	return s, nil
}

// LoadFileFromDisk loads and analyzes an .xlsx file from disk.
func (s *AppServer) LoadFileFromDisk(p string) error {
	raw, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	return s.LoadBytes(filepath.Base(p), raw)
}

// LoadBytes parses and analyzes raw .xlsx bytes and updates the active report.
func (s *AppServer) LoadBytes(fileName string, raw []byte) error {
	wb, err := xlsx.OpenBytes(raw)
	if err != nil {
		return err
	}
	rep := analytics.AnalyzeWorkbook(fileName, wb)
	s.mu.Lock()
	s.currentWB = wb
	s.currentRep = rep
	s.mu.Unlock()
	return nil
}

// Handler returns the root http.Handler with gzip compression and permissive preview headers.
func (s *AppServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/dashboard", s.handleDashboard)
	mux.HandleFunc("/api/upload", s.handleUpload)
	mux.HandleFunc("/api/open-dialog", s.handleOpenDialog)
	mux.HandleFunc("/api/reset", s.handleReset)
	mux.HandleFunc("/api/export/csv", s.handleExportCSV)
	mux.HandleFunc("/api/ping", s.handlePing)
	mux.HandleFunc("/api/window-closed", s.handleWindowClosed)
	mux.HandleFunc("/api/shutdown", s.handleShutdown)

	if s.opts.WebFS != nil {
		sub, err := fs.Sub(s.opts.WebFS, "web")
		if err == nil {
			mux.Handle("/", http.FileServer(http.FS(sub)))
		}
	}

	return withMiddleware(mux)
}

func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") && strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Encoding", "gzip")
			gz := gzip.NewWriter(w)
			defer gz.Close()
			next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, Writer: gz}, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	return g.Writer.Write(b)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *AppServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	fn := ""
	if s.currentRep != nil {
		fn = s.currentRep.FileName
	}
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                true,
		"fileName":          fn,
		"desktopMode":       s.opts.DesktopMode,
		"hasNativeDialog":   s.opts.NativePicker != nil,
		"hasEmbeddedSample": s.defaultWB != nil,
	})
}

func (s *AppServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	s.recordPing()
	s.mu.RLock()
	rep := s.currentRep
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"report":          rep,
		"desktopMode":     s.opts.DesktopMode,
		"hasNativeDialog": s.opts.NativePicker != nil,
	})
}

func (s *AppServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "требуется метод POST"})
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось прочитать загружаемый файл: " + err.Error()})
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "файл не передан: " + err.Error()})
		return
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 64<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ошибка чтения файла: " + err.Error()})
		return
	}
	if err := s.LoadBytes(hdr.Filename, raw); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ошибка разбора Excel (.xlsx): " + err.Error()})
		return
	}
	s.mu.RLock()
	rep := s.currentRep
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"report": rep})
}

func (s *AppServer) handleOpenDialog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "требуется метод POST"})
		return
	}
	if s.opts.NativePicker == nil {
		writeJSON(w, http.StatusOK, map[string]any{"supported": false})
		return
	}
	chosen, err := s.opts.NativePicker()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if chosen == "" {
		writeJSON(w, http.StatusOK, map[string]any{"supported": true, "cancelled": true})
		return
	}
	if err := s.LoadFileFromDisk(chosen); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось загрузить Excel: " + err.Error()})
		return
	}
	s.mu.RLock()
	rep := s.currentRep
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"supported": true,
		"report":    rep,
	})
}

func (s *AppServer) handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "требуется метод POST"})
		return
	}
	if s.defaultWB == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "базовый файл отсутствует"})
		return
	}
	rep := analytics.AnalyzeWorkbook(s.defaultName, s.defaultWB)
	s.mu.Lock()
	s.currentWB = s.defaultWB
	s.currentRep = rep
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"report": rep})
}

func (s *AppServer) handlePing(w http.ResponseWriter, r *http.Request) {
	s.recordPing()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *AppServer) handleWindowClosed(w http.ResponseWriter, r *http.Request) {
	s.lifeMu.Lock()
	s.windowClosing = true
	s.lifeMu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (s *AppServer) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if !s.opts.DesktopMode {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "доступно только в настольном режиме"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"shuttingDown": true})
	if s.opts.OnShutdown != nil {
		go func() {
			time.Sleep(200 * time.Millisecond)
			s.opts.OnShutdown()
		}()
	}
}

func (s *AppServer) recordPing() {
	s.lifeMu.Lock()
	s.hasConnected = true
	s.windowClosing = false
	s.lastPing = time.Now()
	s.lifeMu.Unlock()
}

func (s *AppServer) monitorDesktopLifecycle() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		s.lifeMu.Lock()
		connected := s.hasConnected
		closing := s.windowClosing
		idle := time.Since(s.lastPing)
		s.lifeMu.Unlock()

		if !connected {
			continue
		}
		if (closing && idle > 4*time.Second) || idle > 25*time.Second {
			s.opts.OnShutdown()
			return
		}
	}
}

func (s *AppServer) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	section := r.URL.Query().Get("section")
	sheetName := r.URL.Query().Get("name")

	s.mu.RLock()
	rep := s.currentRep
	wb := s.currentWB
	s.mu.RUnlock()

	var buf bytes.Buffer
	// Write UTF-8 BOM so Russian Excel on Windows opens Cyrillic & semicolon columns cleanly
	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	fileName := "sales_analytics_export.csv"
	writeRow := func(cols ...string) {
		escaped := make([]string, len(cols))
		for i, col := range cols {
			c := strings.ReplaceAll(col, "\"", "\"\"")
			if strings.ContainsAny(c, ";\"\r\n") {
				c = "\"" + c + "\""
			}
			escaped[i] = c
		}
		buf.WriteString(strings.Join(escaped, ";") + "\r\n")
	}
	fmtNum := func(v float64) string {
		if math.Abs(v-math.Round(v)) < 1e-6 {
			return strconv.FormatInt(int64(math.Round(v)), 10)
		}
		return strings.ReplaceAll(strconv.FormatFloat(v, 'f', 2, 64), ".", ",")
	}

	switch section {
	case "karcher":
		fileName = "karcher_sales_purchases_18m.csv"
		writeRow("Период", "Полугодие", "Реализация (BYN без НДС)", "Закупки (BYN без НДС)", "Разница Закупки-Реализация (BYN)", "Накопленное сальдо (BYN)", "Закупки к продажам (%)")
		for _, m := range rep.Karcher.Monthly {
			writeRow(m.Month, m.HalfYear, fmtNum(m.SalesBYN), fmtNum(m.PurchasesBYN), fmtNum(m.DiffBYN), fmtNum(m.CumDiffBYN), fmtNum(m.PurchToSales))
		}
	case "retail":
		fileName = "retail_stores_analytics.csv"
		writeRow("Город", "Магазин", "Продажи 15 мес. (BYN)", "Доля продаж (%)", "Продажи 1 кв. 2023", "Продажи 1 кв. 2024", "Рост YoY (%)", "Средние продажи 6 мес.", "Остатки 31.03.2024", "Остатки 30.06.2024", "Доля Керхер (%)", "Запас (мес.)")
		for _, st := range rep.Retail.Stores {
			writeRow(st.City, st.StoreName, fmtNum(st.TotalSales15m), fmtNum(st.SalesSharePct), fmtNum(st.SalesQ1_2023), fmtNum(st.SalesQ1_2024), fmtNum(st.YoYGrowthQ1Pct), fmtNum(st.Avg6mSales), fmtNum(st.Stock24_03), fmtNum(st.Stock24_06), fmtNum(st.KarcherSharePct), fmtNum(st.StockMonths))
		}
	case "groups":
		fileName = "groups_abc_inventory.csv"
		writeRow("Группа / Поставщик", "ABC-класс", "Остатки 31.12.2023 (BYN)", "Остатки 30.06.2024 (BYN)", "Изменение (BYN)", "Изменение (%)", "Доля (%)", "Накопленная доля (%)")
		for _, g := range rep.Groups.Items {
			writeRow(g.Supplier, g.ABCClass, fmtNum(g.Stock2023), fmtNum(g.Stock2024), fmtNum(g.Delta), fmtNum(g.DeltaPct), fmtNum(g.SharePct), fmtNum(g.CumShare))
		}
	case "warehouses":
		fileName = "warehouses_channels.csv"
		writeRow("Склад / Подразделение", "Канал сбыта", "Остатки "+rep.Warehouses.DateStart+" (BYN)", "Остатки "+rep.Warehouses.DateEnd+" (BYN)", "Изменение (BYN)", "Изменение (%)", "Доля (%)")
		for _, wItem := range rep.Warehouses.Items {
			writeRow(wItem.Name, wItem.Channel, fmtNum(wItem.StockStart), fmtNum(wItem.StockEnd), fmtNum(wItem.Delta), fmtNum(wItem.DeltaPct), fmtNum(wItem.ShareEnd))
		}
	case "intercompany":
		fileName = "smg_to_chst_export.csv"
		writeRow("Группа / Поставщик", "ABC", "4 кв. 2023 (BYN)", "1 кв. 2024 (BYN)", "Динамика (BYN)", "Динамика (%)", "Доля 1 кв. 2024 (%)", "Накопленная доля (%)")
		for _, it := range rep.Intercompany.Items {
			writeRow(it.Supplier, it.ABCClass, fmtNum(it.Q4_2023), fmtNum(it.Q1_2024), fmtNum(it.Delta), fmtNum(it.DeltaPct), fmtNum(it.ShareQ1), fmtNum(it.CumShare))
		}
	case "prepayments":
		fileName = "prepayments_commitments.csv"
		writeRow("Статус", "Поставщик / Группа", "Сумма в валюте", "Валюта", "Таможня в валюте", "Закупка (BYN)", "Таможня (BYN)", "Всего с таможней (BYN)")
		for _, p := range rep.Prepayments.PaidOrders {
			writeRow("Оплачено (в пути)", p.Supplier, fmtNum(p.AmountOrig), p.Currency, fmtNum(p.CustomsOrig), fmtNum(p.PurchaseBYN), fmtNum(p.CustomsBYN), fmtNum(p.TotalBYN))
		}
		for _, p := range rep.Prepayments.UnpaidOrders {
			writeRow("К оплате", p.Supplier, fmtNum(p.AmountOrig), p.Currency, fmtNum(p.CustomsOrig), fmtNum(p.PurchaseBYN), fmtNum(p.CustomsBYN), fmtNum(p.TotalBYN))
		}
		if rep.Prepayments.KarcherRFDebtBYN > 0 {
			writeRow("Задолженность", "Керхер РФ", fmtNum(rep.Prepayments.KarcherRFDebtRUB), "росс.руб", "0", fmtNum(rep.Prepayments.KarcherRFDebtBYN), "0", fmtNum(rep.Prepayments.KarcherRFDebtBYN))
		}
	case "rf1c":
		fileName = "rf_procurement_1c_items.csv"
		writeRow("Поставщик", "Категория", "Документ", "Дата", "Месяц", "Номенклатура (SKU)", "НДС", "Количество", "Сумма без НДС (RUB)", "Сумма НДС (RUB)", "Сумма с НДС (RUB)")
		for _, li := range rep.RFProcurement.LineItems {
			writeRow(li.Supplier, li.Category, li.DocNumber, li.DocDate, li.Month, li.SKU, li.VATRate, fmtNum(li.Qty), fmtNum(li.SumNoVAT), fmtNum(li.SumVAT), fmtNum(li.SumWithVAT))
		}
	case "sheet":
		sh := wb.Sheet(sheetName)
		if sh == nil && len(wb.Sheets) > 0 {
			sh = wb.Sheets[0]
		}
		if sh != nil {
			fileName = "sheet_export.csv"
			for r := 1; r <= sh.MaxRow; r++ {
				rowOut := make([]string, sh.MaxCol)
				for c := 1; c <= sh.MaxCol; c++ {
					cell := sh.Cell(r, c)
					if cell.HasFloat && !cell.IsDate {
						rowOut[c-1] = fmtNum(cell.FloatVal)
					} else {
						rowOut[c-1] = strings.TrimSpace(cell.Text)
					}
				}
				writeRow(rowOut...)
			}
		}
	default:
		fileName = "sales_monthly_summary.csv"
		writeRow("Полугодие", "Месяц", "Реализация всего (BYN без НДС)", "В т.ч. Керхер (BYN)", "Прочий ассортимент (BYN)", "Доля Керхер (%)", "Изменение к пред. мес. (%)")
		for _, m := range rep.SalesInventory.MonthlySales {
			writeRow(m.Period, m.FullLabel, fmtNum(m.TotalSales), fmtNum(m.KarcherSales), fmtNum(m.OtherSales), fmtNum(m.KarcherShare), fmtNum(m.MoMGrowthPct))
		}
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	_, _ = w.Write(buf.Bytes())
}
