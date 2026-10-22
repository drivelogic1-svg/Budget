package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

// Workbook represents a parsed OpenXML (.xlsx) spreadsheet workbook.
type Workbook struct {
	Sheets    []*Sheet
	byName    map[string]*Sheet
	byNormKey map[string]*Sheet
}

// Sheet represents a single worksheet inside an .xlsx workbook.
type Sheet struct {
	Name        string
	MaxRow      int
	MaxCol      int
	Rows        map[int]map[int]Cell // 1-based row -> 1-based col -> Cell
	MergedCells []string
}

// Cell represents a parsed Excel cell with both text, numeric, formula, and style metadata.
type Cell struct {
	Ref        string
	Row        int
	Col        int
	RawType    string
	Text       string
	FloatVal   float64
	HasFloat   bool
	Formula    string
	Indent     int
	IsDate     bool
	IsPercent  bool
}

// OpenFile reads and parses an .xlsx file from disk.
func OpenFile(filePath string) (*Workbook, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return OpenBytes(data)
}

// OpenBytes reads and parses an .xlsx archive from memory.
func OpenBytes(data []byte) (*Workbook, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть ZIP/XLSX архив: %w", err)
	}

	files := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		clean := strings.TrimPrefix(f.Name, "/")
		files[clean] = f
	}

	rels, err := parseWorkbookRels(files["xl/_rels/workbook.xml.rels"])
	if err != nil {
		return nil, err
	}

	sharedStrings, err := parseSharedStrings(files["xl/sharedStrings.xml"])
	if err != nil {
		return nil, err
	}

	styles := parseStyles(files["xl/styles.xml"])

	sheetDefs, err := parseWorkbookSheets(files["xl/workbook.xml"])
	if err != nil {
		return nil, err
	}

	wb := &Workbook{
		byName:    make(map[string]*Sheet, len(sheetDefs)),
		byNormKey: make(map[string]*Sheet, len(sheetDefs)),
	}

	for idx, def := range sheetDefs {
		target := rels[def.RelID]
		if target == "" {
			target = fmt.Sprintf("worksheets/sheet%d.xml", idx+1)
		}
		target = strings.TrimPrefix(target, "/")
		if !strings.HasPrefix(target, "xl/") {
			target = path.Join("xl", target)
		}
		zf := files[target]
		if zf == nil {
			continue
		}
		sh, err := parseWorksheet(def.Name, zf, sharedStrings, styles)
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения листа %q: %w", def.Name, err)
		}
		wb.Sheets = append(wb.Sheets, sh)
		wb.byName[sh.Name] = sh
		wb.byNormKey[normSheetKey(sh.Name)] = sh
	}

	return wb, nil
}

func normSheetKey(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(s)), " "))
}

// Sheet returns a worksheet by exact or case-insensitive normalized name.
func (wb *Workbook) Sheet(name string) *Sheet {
	if wb == nil {
		return nil
	}
	if s, ok := wb.byName[name]; ok {
		return s
	}
	return wb.byNormKey[normSheetKey(name)]
}

// SheetNames returns all worksheet names in workbook order.
func (wb *Workbook) SheetNames() []string {
	if wb == nil {
		return nil
	}
	out := make([]string, len(wb.Sheets))
	for i, s := range wb.Sheets {
		out[i] = s.Name
	}
	return out
}

// Cell returns a cell at 1-based row and col. Returns empty Cell if absent.
func (s *Sheet) Cell(row, col int) Cell {
	if s == nil || s.Rows == nil {
		return Cell{Row: row, Col: col}
	}
	if r, ok := s.Rows[row]; ok {
		if c, ok2 := r[col]; ok2 {
			return c
		}
	}
	return Cell{Row: row, Col: col}
}

// CellRef returns a cell by A1-style reference such as "B17" or "AA3".
func (s *Sheet) CellRef(ref string) Cell {
	row, col := ParseCellRef(ref)
	return s.Cell(row, col)
}

// String returns the trimmed text value of a cell at 1-based row and col.
func (s *Sheet) String(row, col int) string {
	return strings.TrimSpace(s.Cell(row, col).Text)
}

// Float returns the float64 value of a cell (0 if empty or non-numeric).
func (s *Sheet) Float(row, col int) float64 {
	v, _ := s.FloatOk(row, col)
	return v
}

// FloatOk returns the numeric value of a cell and whether it holds a valid number.
func (s *Sheet) FloatOk(row, col int) (float64, bool) {
	c := s.Cell(row, col)
	if c.HasFloat && !math.IsNaN(c.FloatVal) && !math.IsInf(c.FloatVal, 0) {
		return c.FloatVal, true
	}
	txt := strings.TrimSpace(c.Text)
	if txt == "" || txt == "-" || txt == "—" {
		return 0, false
	}
	cleaned := strings.ReplaceAll(strings.ReplaceAll(txt, "\u00a0", ""), " ", "")
	cleaned = strings.ReplaceAll(cleaned, ",", ".")
	if v, err := strconv.ParseFloat(cleaned, 64); err == nil {
		return v, true
	}
	return 0, false
}

// ColName converts a 1-based column number to Excel letters (1 -> "A", 27 -> "AA").
func ColName(col int) string {
	if col <= 0 {
		return ""
	}
	var buf [8]byte
	pos := len(buf)
	for col > 0 {
		col--
		pos--
		buf[pos] = byte('A' + (col % 26))
		col /= 26
	}
	return string(buf[pos:])
}

// ParseCellRef parses an Excel cell reference like "C12" into 1-based (row, col).
func ParseCellRef(ref string) (row, col int) {
	ref = strings.TrimSpace(strings.ReplaceAll(ref, "$", ""))
	i := 0
	for i < len(ref) {
		ch := ref[i]
		if ch >= 'A' && ch <= 'Z' {
			col = col*26 + int(ch-'A'+1)
			i++
		} else if ch >= 'a' && ch <= 'z' {
			col = col*26 + int(ch-'a'+1)
			i++
		} else {
			break
		}
	}
	if i < len(ref) {
		row, _ = strconv.Atoi(ref[i:])
	}
	return row, col
}

type wbSheetDef struct {
	Name  string
	RelID string
}

func readZipXML(zf *zip.File) ([]byte, error) {
	if zf == nil {
		return nil, nil
	}
	rc, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func parseWorkbookRels(zf *zip.File) (map[string]string, error) {
	out := make(map[string]string)
	raw, err := readZipXML(zf)
	if err != nil || len(raw) == 0 {
		return out, err
	}
	var doc struct {
		Relationships []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err := xml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("ошибка чтения workbook.xml.rels: %w", err)
	}
	for _, r := range doc.Relationships {
		out[r.ID] = r.Target
	}
	return out, nil
}

func parseWorkbookSheets(zf *zip.File) ([]wbSheetDef, error) {
	raw, err := readZipXML(zf)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("файл xl/workbook.xml не найден")
	}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var defs []wbSheetDef
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения workbook.xml: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "sheet" {
			continue
		}
		var d wbSheetDef
		for _, attr := range se.Attr {
			switch attr.Name.Local {
			case "name":
				d.Name = attr.Value
			case "id":
				d.RelID = attr.Value
			}
		}
		if d.Name != "" {
			defs = append(defs, d)
		}
	}
	return defs, nil
}

func parseSharedStrings(zf *zip.File) ([]string, error) {
	raw, err := readZipXML(zf)
	if err != nil || len(raw) == 0 {
		return nil, err
	}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var result []string
	var inSI bool
	var sb strings.Builder
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения sharedStrings.xml: %w", err)
		}
		switch el := tok.(type) {
		case xml.StartElement:
			if el.Name.Local == "si" {
				inSI = true
				sb.Reset()
			} else if inSI && el.Name.Local == "t" {
				var text string
				if err := dec.DecodeElement(&text, &el); err != nil {
					return nil, err
				}
				sb.WriteString(text)
			}
		case xml.EndElement:
			if el.Name.Local == "si" && inSI {
				result = append(result, sb.String())
				inSI = false
			}
		}
	}
	return result, nil
}

type styleInfo struct {
	Indent    int
	IsDate    bool
	IsPercent bool
}

func parseStyles(zf *zip.File) []styleInfo {
	raw, err := readZipXML(zf)
	if err != nil || len(raw) == 0 {
		return nil
	}
	var doc struct {
		NumFmts struct {
			Items []struct {
				ID   int    `xml:"numFmtId,attr"`
				Code string `xml:"formatCode,attr"`
			} `xml:"numFmt"`
		} `xml:"numFmts"`
		CellXfs struct {
			Xfs []struct {
				NumFmtID  int `xml:"numFmtId,attr"`
				Alignment struct {
					Indent int `xml:"indent,attr"`
				} `xml:"alignment"`
			} `xml:"xf"`
		} `xml:"cellXfs"`
	}
	if err := xml.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	customFmt := make(map[int]string, len(doc.NumFmts.Items))
	for _, nf := range doc.NumFmts.Items {
		customFmt[nf.ID] = strings.ToLower(nf.Code)
	}

	out := make([]styleInfo, len(doc.CellXfs.Xfs))
	for i, xf := range doc.CellXfs.Xfs {
		id := xf.NumFmtID
		code := customFmt[id]
		isDate := (id >= 14 && id <= 22) || (id >= 45 && id <= 47)
		if !isDate && code != "" {
			if (strings.Contains(code, "yy") || strings.Contains(code, "dd") || strings.Contains(code, "гг") || strings.Contains(code, "дд")) &&
				!strings.Contains(code, "#") && !strings.Contains(code, "0.0") {
				isDate = true
			}
		}
		isPct := id == 9 || id == 10 || strings.Contains(code, "%")
		out[i] = styleInfo{
			Indent:    xf.Alignment.Indent,
			IsDate:    isDate,
			IsPercent: isPct,
		}
	}
	return out
}

func excelSerialToDateString(serial float64) string {
	// Excel Windows 1900 date system (including 1900 leap-year bug offset -> 1899-12-30)
	days := int(math.Floor(serial + 1e-6))
	if days < 1 || days > 100000 {
		return strconv.FormatFloat(serial, 'f', -1, 64)
	}
	base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	dt := base.AddDate(0, 0, days)
	return dt.Format("02.01.2006")
}

type xmlRow struct {
	R     int       `xml:"r,attr"`
	Cells []xmlCell `xml:"c"`
}

type xmlCell struct {
	Ref       string `xml:"r,attr"`
	StyleIdx  int    `xml:"s,attr"`
	Type      string `xml:"t,attr"`
	Formula   string `xml:"f"`
	Value     string `xml:"v"`
	InlineStr struct {
		T    string `xml:"t"`
		Runs []struct {
			T string `xml:"t"`
		} `xml:"r"`
	} `xml:"is"`
}

func parseWorksheet(name string, zf *zip.File, sst []string, styles []styleInfo) (*Sheet, error) {
	raw, err := readZipXML(zf)
	if err != nil {
		return nil, err
	}
	sh := &Sheet{
		Name: name,
		Rows: make(map[int]map[int]Cell),
	}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	curRow := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "row":
			var xr xmlRow
			if err := dec.DecodeElement(&xr, &se); err != nil {
				return nil, err
			}
			if xr.R > 0 {
				curRow = xr.R
			} else {
				curRow++
			}
			curCol := 0
			for _, xc := range xr.Cells {
				r, c := ParseCellRef(xc.Ref)
				if r == 0 {
					r = curRow
				}
				if c == 0 {
					curCol++
					c = curCol
				} else {
					curCol = c
				}
				cell := Cell{
					Ref:     xc.Ref,
					Row:     r,
					Col:     c,
					RawType: xc.Type,
					Formula: xc.Formula,
				}
				if cell.Ref == "" {
					cell.Ref = ColName(c) + strconv.Itoa(r)
				}
				if xc.StyleIdx >= 0 && xc.StyleIdx < len(styles) {
					st := styles[xc.StyleIdx]
					cell.Indent = st.Indent
					cell.IsDate = st.IsDate
					cell.IsPercent = st.IsPercent
				}

				switch xc.Type {
				case "s":
					idx, err := strconv.Atoi(strings.TrimSpace(xc.Value))
					if err == nil && idx >= 0 && idx < len(sst) {
						cell.Text = sst[idx]
					}
				case "inlineStr":
					if len(xc.InlineStr.Runs) > 0 {
						var sb strings.Builder
						for _, run := range xc.InlineStr.Runs {
							sb.WriteString(run.T)
						}
						cell.Text = sb.String()
					} else {
						cell.Text = xc.InlineStr.T
					}
				case "str", "e":
					cell.Text = xc.Value
				case "b":
					if strings.TrimSpace(xc.Value) == "1" {
						cell.Text = "TRUE"
					} else {
						cell.Text = "FALSE"
					}
				default:
					valStr := strings.TrimSpace(xc.Value)
					if valStr != "" {
						if fv, err := strconv.ParseFloat(valStr, 64); err == nil {
							cell.FloatVal = fv
							cell.HasFloat = true
							if cell.IsDate {
								cell.Text = excelSerialToDateString(fv)
							} else {
								cell.Text = valStr
							}
						} else {
							cell.Text = valStr
						}
					}
				}

				if cell.Text != "" || cell.HasFloat || cell.Formula != "" {
					rowMap := sh.Rows[r]
					if rowMap == nil {
						rowMap = make(map[int]Cell)
						sh.Rows[r] = rowMap
					}
					rowMap[c] = cell
					if r > sh.MaxRow {
						sh.MaxRow = r
					}
					if c > sh.MaxCol {
						sh.MaxCol = c
					}
				}
			}
		case "mergeCell":
			for _, a := range se.Attr {
				if a.Name.Local == "ref" && a.Value != "" {
					sh.MergedCells = append(sh.MergedCells, a.Value)
				}
			}
		}
	}
	return sh, nil
}
