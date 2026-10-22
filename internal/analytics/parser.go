package analytics

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/drivelogic1-svg/Budget/internal/xlsx"
)

var docDateRegex = regexp.MustCompile(`от\s+(\d{2})\.(\d{2})\.(\d{4})`)
var docNumRegex = regexp.MustCompile(`([А-ЯA-Z0-9-]+)\s+от\s+\d{2}\.\d{2}\.\d{4}`)

// AnalyzeWorkbook parses an xlsx.Workbook into a comprehensive DashboardReport.
func AnalyzeWorkbook(fileName string, wb *xlsx.Workbook) *DashboardReport {
	rep := &DashboardReport{
		FileName:   fileName,
		LoadedAt:   time.Now().Format("02.01.2006 15:04:05"),
		SheetCount: len(wb.Sheets),
		SheetNames: wb.SheetNames(),
	}

	hasStockTotal := wb.Sheet("Остатки всего") != nil
	hasKarcher := wb.Sheet("Керхер") != nil
	hasRetail := wb.Sheet("Розница") != nil
	hasGroups := wb.Sheet("Группы") != nil
	rep.IsBudgetDataset = hasStockTotal || hasKarcher || hasRetail || hasGroups

	parseKarcher(wb, rep)
	parseSalesAndStock(wb, rep)
	parseWarehousesAndRB(wb, rep)
	parseRetail(wb, rep)
	parseGroupsAndRF(wb, rep)
	parseIntercompany(wb, rep)
	parsePrepayments(wb, rep)
	parseRFProcurement(wb, rep)
	buildOverviewKPIs(rep)
	buildExecutiveInsights(rep)
	buildRawAndGenericSheets(wb, rep)

	return rep
}

func pctChange(curr, prev float64) float64 {
	if math.Abs(prev) < 1e-9 {
		return 0
	}
	return (curr/prev - 1.0) * 100.0
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func parseKarcher(wb *xlsx.Workbook, rep *DashboardReport) {
	sh := wb.Sheet("Керхер")
	if sh == nil {
		return
	}
	var kd KarcherData

	// Parse 18-month table K11:N28
	cum := 0.0
	for r := 11; r <= sh.MaxRow; r++ {
		mName := cleanText(sh.String(r, 11))
		if mName == "" {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(mName), "ИТОГО") {
			break
		}
		sales, okS := sh.FloatOk(r, 12)
		purch, okP := sh.FloatOk(r, 13)
		if !okS && !okP {
			continue
		}
		diff := purch - sales
		cum += diff
		halfYear := classifyMonthHalfYear(mName)
		ratio := 0.0
		if sales > 0 {
			ratio = (purch / sales) * 100.0
		}
		kd.Monthly = append(kd.Monthly, KarcherMonthRow{
			Month:        mName,
			HalfYear:     halfYear,
			SalesBYN:     round2(sales),
			PurchasesBYN: round2(purch),
			DiffBYN:      round2(diff),
			CumDiffBYN:   round2(cum),
			PurchToSales: round2(ratio),
		})
		kd.TotalSales18m += sales
		kd.TotalPurch18m += purch
		kd.TotalDiff18m += diff

		switch halfYear {
		case "1П 2023":
			kd.SalesH1_2023 += sales
		case "2П 2023":
			kd.SalesH2_2023 += sales
		case "1П 2024":
			kd.SalesH1_2024 += sales
			kd.PurchH1_2024 += purch
		}
	}
	kd.TotalSales18m = round2(kd.TotalSales18m)
	kd.TotalPurch18m = round2(kd.TotalPurch18m)
	kd.TotalDiff18m = round2(kd.TotalDiff18m)
	kd.SalesH1_2023 = round2(kd.SalesH1_2023)
	kd.SalesH2_2023 = round2(kd.SalesH2_2023)
	kd.SalesH1_2024 = round2(kd.SalesH1_2024)
	kd.PurchH1_2024 = round2(kd.PurchH1_2024)
	kd.NetBuildH1_2024 = round2(kd.PurchH1_2024 - kd.SalesH1_2024)

	// Parse H1 2023 (rows 8-9 cols 2..7) and H2 2023 (rows 3-4 cols 2..7) segment breakdown
	h1Months := []string{"Январь 2023", "Февраль 2023", "Март 2023", "Апрель 2023", "Май 2023", "Июнь 2023"}
	for idx, col := range []int{2, 3, 4, 5, 6, 7} {
		hg := sh.Float(8, col)
		prof := sh.Float(9, col)
		tot := hg + prof
		if tot > 0 {
			kd.Segments2023 = append(kd.Segments2023, KarcherSegmentRow{
				Month:     h1Months[idx],
				HalfYear:  "1П 2023",
				HG:        round2(hg),
				Prof:      round2(prof),
				Total:     round2(tot),
				HGShare:   round2(hg / tot * 100),
				ProfShare: round2(prof / tot * 100),
			})
		}
	}
	h2Months := []string{"Июль 2023", "Август 2023", "Сентябрь 2023", "Октябрь 2023", "Ноябрь 2023", "Декабрь 2023"}
	for idx, col := range []int{2, 3, 4, 5, 6, 7} {
		hg := sh.Float(3, col)
		prof := sh.Float(4, col)
		tot := hg + prof
		if tot > 0 {
			lbl := cleanText(sh.String(2, col))
			if lbl == "" {
				lbl = h2Months[idx]
			}
			kd.Segments2023 = append(kd.Segments2023, KarcherSegmentRow{
				Month:     lbl,
				HalfYear:  "2П 2023",
				HG:        round2(hg),
				Prof:      round2(prof),
				Total:     round2(tot),
				HGShare:   round2(hg / tot * 100),
				ProfShare: round2(prof / tot * 100),
			})
		}
	}

	// Extra note at bottom (e.g. row 35: 137123.08 "продано Керхер РФ с 21.06.")
	for r := 30; r <= sh.MaxRow; r++ {
		if v, ok := sh.FloatOk(r, 11); ok && v > 0 {
			kd.SoldToKarcherRF = round2(v)
			kd.SoldRFNote = cleanText(sh.String(r, 12))
			break
		}
	}

	// Pull Karcher stock from Группы if available
	if sg := wb.Sheet("Группы"); sg != nil {
		for r := 2; r <= sg.MaxRow; r++ {
			name := strings.ToUpper(cleanText(sg.String(r, 2)))
			if strings.Contains(name, "КЕРХЕР") && strings.Contains(name, "H&G") {
				kd.StockHG_End2023 = round2(sg.Float(r, 3))
				kd.StockHG_Mid2024 = round2(sg.Float(r, 4))
			} else if strings.Contains(name, "КЕРХЕР") && strings.Contains(name, "ПРОФ") {
				kd.StockProfEnd2023 = round2(sg.Float(r, 3))
				kd.StockProfMid2024 = round2(sg.Float(r, 4))
			}
		}
	}

	rep.Karcher = kd
}

func classifyMonthHalfYear(m string) string {
	ml := strings.ToLower(m)
	is2024 := strings.Contains(ml, "2024")
	is2023 := strings.Contains(ml, "2023")
	firstHalfMonths := []string{"январ", "феврал", "март", "апрел", "май", "июн"}
	for _, p := range firstHalfMonths {
		if strings.Contains(ml, p) {
			if is2024 {
				return "1П 2024"
			}
			if is2023 {
				return "1П 2023"
			}
		}
	}
	if is2023 {
		return "2П 2023"
	}
	if is2024 {
		return "2П 2024"
	}
	return ""
}

func parseSalesAndStock(wb *xlsx.Workbook, rep *DashboardReport) {
	sh := wb.Sheet("Остатки всего")
	if sh == nil {
		return
	}

	var data SalesInventoryData
	// Stock checkpoints rows 2..5
	dateRegex := regexp.MustCompile(`\d{2}\.\d{2}\.\d{4}`)
	var prevVal float64
	for r := 2; r <= 10 && r <= sh.MaxRow; r++ {
		lbl := cleanText(sh.String(r, 1))
		val, ok := sh.FloatOk(r, 2)
		if !ok || lbl == "" {
			continue
		}
		low := strings.ToLower(lbl)
		if strings.Contains(low, "увеличение") || strings.Contains(low, "разница") || strings.Contains(low, "%") {
			continue
		}
		dt := dateRegex.FindString(lbl)
		if dt == "" {
			dt = fmt.Sprintf("Точка %d", len(data.StockCheckpoints)+1)
		}
		delta := 0.0
		dpct := 0.0
		if len(data.StockCheckpoints) > 0 && prevVal > 0 {
			delta = round2(val - prevVal)
			dpct = round2(pctChange(val, prevVal))
		}
		data.StockCheckpoints = append(data.StockCheckpoints, StockCheckpoint{
			DateLabel: dt,
			Label:     lbl,
			ValueBYN:  round2(val),
			DeltaPrev: delta,
			DeltaPct:  dpct,
		})
		prevVal = val
	}

	// Map Karcher monthly sales by lowercase month+year
	karcherByKey := make(map[string]float64)
	for _, km := range rep.Karcher.Monthly {
		karcherByKey[strings.ToLower(km.Month)] = km.SalesBYN
	}

	// Monthly sales rows 12..24
	h2Set := map[string]bool{"июль": true, "август": true, "сентябрь": true, "октябрь": true, "ноябрь": true, "декабрь": true}
	var prevSale float64
	for r := 11; r <= sh.MaxRow; r++ {
		mRaw := cleanText(sh.String(r, 2))
		val, ok := sh.FloatOk(r, 3)
		if mRaw == "" || !ok {
			continue
		}
		mLow := strings.ToLower(mRaw)
		period := "1П 2024"
		yearSuffix := "2024"
		if h2Set[mLow] {
			period = "2П 2023"
			yearSuffix = "2023"
		}
		titleMonth := capitalizeRU(mLow)
		fullLabel := titleMonth + " " + yearSuffix
		kSale := karcherByKey[strings.ToLower(fullLabel)]
		other := val - kSale
		if other < 0 {
			other = 0
		}
		kShare := 0.0
		if val > 0 && kSale > 0 {
			kShare = kSale / val * 100.0
		}
		mom := 0.0
		if len(data.MonthlySales) > 0 && prevSale > 0 {
			mom = pctChange(val, prevSale)
		}
		data.MonthlySales = append(data.MonthlySales, MonthlySalePoint{
			Period:       period,
			Month:        titleMonth,
			FullLabel:    fullLabel,
			TotalSales:   round2(val),
			KarcherSales: round2(kSale),
			OtherSales:   round2(other),
			KarcherShare: round2(kShare),
			MoMGrowthPct: round2(mom),
		})
		prevSale = val
	}

	rep.SalesInventory = data
}

func capitalizeRU(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	first := strings.ToUpper(string(runes[0]))
	return first + string(runes[1:])
}

func parseWarehousesAndRB(wb *xlsx.Workbook, rep *DashboardReport) {
	var wd WarehousesData
	if sh := wb.Sheet("Склады"); sh != nil {
		wd.DateStart = cleanText(sh.String(1, 2))
		wd.DateEnd = cleanText(sh.String(1, 3))
		if wd.DateStart == "" {
			wd.DateStart = "31.12.2023"
		}
		if wd.DateEnd == "" {
			wd.DateEnd = "30.06.2024"
		}

		channelMap := make(map[string]*WarehouseChannelAgg)
		channelOrder := []string{"Центральные склады", "Розничные магазины", "Отделы продаж B2B", "Сервис и производство"}
		for _, ch := range channelOrder {
			channelMap[ch] = &WarehouseChannelAgg{Channel: ch}
		}

		for r := 2; r <= sh.MaxRow; r++ {
			name := cleanText(sh.String(r, 1))
			if name == "" {
				continue
			}
			vStart := sh.Float(r, 2)
			vEnd := sh.Float(r, 3)
			if vStart == 0 && vEnd == 0 {
				continue
			}
			ch := classifyWarehouseChannel(name)
			delta := vEnd - vStart
			wd.Items = append(wd.Items, WarehouseRow{
				Name:       name,
				Channel:    ch,
				StockStart: round2(vStart),
				StockEnd:   round2(vEnd),
				Delta:      round2(delta),
				DeltaPct:   round2(pctChange(vEnd, vStart)),
			})
			wd.TotalStart += vStart
			wd.TotalEnd += vEnd

			agg := channelMap[ch]
			if agg == nil {
				agg = &WarehouseChannelAgg{Channel: ch}
				channelMap[ch] = agg
				channelOrder = append(channelOrder, ch)
			}
			agg.StockStart += vStart
			agg.StockEnd += vEnd
		}

		wd.TotalStart = round2(wd.TotalStart)
		wd.TotalEnd = round2(wd.TotalEnd)
		wd.TotalDelta = round2(wd.TotalEnd - wd.TotalStart)
		wd.TotalDeltaPct = round2(pctChange(wd.TotalEnd, wd.TotalStart))

		for i := range wd.Items {
			if wd.TotalEnd > 0 {
				wd.Items[i].ShareEnd = round2(wd.Items[i].StockEnd / wd.TotalEnd * 100.0)
			}
		}
		for _, ch := range channelOrder {
			agg := channelMap[ch]
			if agg == nil || (agg.StockStart == 0 && agg.StockEnd == 0) {
				continue
			}
			agg.StockStart = round2(agg.StockStart)
			agg.StockEnd = round2(agg.StockEnd)
			agg.Delta = round2(agg.StockEnd - agg.StockStart)
			agg.DeltaPct = round2(pctChange(agg.StockEnd, agg.StockStart))
			if wd.TotalEnd > 0 {
				agg.ShareEnd = round2(agg.StockEnd / wd.TotalEnd * 100.0)
			}
			wd.ChannelSummary = append(wd.ChannelSummary, *agg)
		}
	}

	if srb := wb.Sheet("РБ"); srb != nil {
		for r := 3; r <= 20 && r <= srb.MaxRow; r++ {
			unit := cleanText(srb.String(r, 1))
			if unit == "" {
				continue
			}
			v23 := srb.Float(r, 2)
			v24 := srb.Float(r, 3)
			if v23 == 0 && v24 == 0 {
				continue
			}
			wd.RBServiceRows = append(wd.RBServiceRows, RBServiceRow{
				Unit:     unit,
				Val2023:  round2(v23),
				Val2024:  round2(v24),
				Delta:    round2(v24 - v23),
				DeltaPct: round2(pctChange(v24, v23)),
			})
		}
	}

	rep.Warehouses = wd
}

func classifyWarehouseChannel(name string) string {
	low := strings.ToLower(name)
	switch {
	case strings.Contains(low, "розн"):
		return "Розничные магазины"
	case strings.Contains(low, "отдел продаж"):
		return "Отделы продаж B2B"
	case strings.HasPrefix(low, "гс ") || strings.Contains(low, "малиновка") || strings.Contains(low, "уручье"):
		return "Центральные склады"
	default:
		return "Сервис и производство"
	}
}

func parseRetail(wb *xlsx.Workbook, rep *DashboardReport) {
	sh := wb.Sheet("Розница")
	if sh == nil {
		return
	}
	var rd RetailData

	// Map 30.06.2024 retail store stock from Warehouses
	stock24_06ByCity := make(map[string]float64)
	for _, w := range rep.Warehouses.Items {
		if w.Channel == "Розничные магазины" {
			city := extractCityName(w.Name)
			stock24_06ByCity[city] = w.StockEnd
		}
	}

	// Karcher retail stock rows 3..6
	karcher24_03ByCity := make(map[string]float64)
	for r := 3; r <= 6 && r <= sh.MaxRow; r++ {
		sName := cleanText(sh.String(r, 1))
		if sName == "" {
			continue
		}
		city := extractCityName(sName)
		karcher24_03ByCity[city] = sh.Float(r, 5)
	}

	// Monthly retail sales rows 22..36 (Cols: B=Витебск, C=Гродно, D=Гомель, E=Брест)
	for r := 22; r <= 36 && r <= sh.MaxRow; r++ {
		per := cleanText(sh.String(r, 1))
		if per == "" || strings.Contains(strings.ToLower(per), "средн") {
			break
		}
		// Normalize "Март" -> "Март 2023", "Апрель" -> "Апрель 2023", "Сентярь 2023" -> "Сентябрь 2023"
		per = normalizeRetailPeriod(per, r)
		vit := sh.Float(r, 2)
		gro := sh.Float(r, 3)
		gom := sh.Float(r, 4)
		bre := sh.Float(r, 5)
		tot := vit + gro + gom + bre
		if tot == 0 {
			continue
		}
		rd.Monthly = append(rd.Monthly, RetailMonthRow{
			Period:  per,
			Vitebsk: round2(vit),
			Grodno:  round2(gro),
			Gomel:   round2(gom),
			Brest:   round2(bre),
			Total:   round2(tot),
		})
		rd.TotalSales15m += tot
	}
	rd.TotalSales15m = round2(rd.TotalSales15m)

	// Aggregate per-city sales metrics from Monthly
	type cityAgg struct {
		total15m float64
		q1_2023  float64
		q1_2024  float64
		last6m   float64
		last6Cnt int
	}
	aggs := map[string]*cityAgg{
		"Брест":   {},
		"Витебск": {},
		"Гомель":  {},
		"Гродно":  {},
	}
	nMonths := len(rd.Monthly)
	for idx, m := range rd.Monthly {
		cityVals := map[string]float64{
			"Витебск": m.Vitebsk,
			"Гродно":  m.Grodno,
			"Гомель":  m.Gomel,
			"Брест":   m.Brest,
		}
		isQ1_23 := idx < 3
		isQ1_24 := strings.HasSuffix(m.Period, "2024")
		isLast6 := idx >= nMonths-6
		for cName, v := range cityVals {
			a := aggs[cName]
			a.total15m += v
			if isQ1_23 {
				a.q1_2023 += v
			}
			if isQ1_24 {
				a.q1_2024 += v
			}
			if isLast6 {
				a.last6m += v
				a.last6Cnt++
			}
		}
	}

	// Full assortment retail store stock rows 11..14
	totalAvg6m := 0.0
	for r := 11; r <= 14 && r <= sh.MaxRow; r++ {
		sName := cleanText(sh.String(r, 1))
		if sName == "" || strings.HasPrefix(strings.ToUpper(sName), "ИТОГО") {
			continue
		}
		city := extractCityName(sName)
		s2212 := sh.Float(r, 2)
		s2306 := sh.Float(r, 3)
		s2312 := sh.Float(r, 4)
		s2403 := sh.Float(r, 5)
		s2406 := stock24_06ByCity[city]
		if s2406 == 0 {
			s2406 = s2403
		}
		k2403 := karcher24_03ByCity[city]
		kShare := 0.0
		if s2403 > 0 {
			kShare = k2403 / s2403 * 100.0
		}

		ca := aggs[city]
		if ca == nil {
			ca = &cityAgg{}
		}
		avg6 := 0.0
		if ca.last6Cnt > 0 {
			avg6 = ca.last6m / float64(ca.last6Cnt)
		}
		totalAvg6m += avg6
		months := 0.0
		if avg6 > 0 {
			months = s2403 / avg6
		}
		status := "optimal"
		if months > 3.0 {
			status = "high"
		} else if months >= 2.5 {
			status = "moderate"
		}
		salesShare := 0.0
		if rd.TotalSales15m > 0 {
			salesShare = ca.total15m / rd.TotalSales15m * 100.0
		}

		rd.Stores = append(rd.Stores, RetailStoreSummary{
			City:            city,
			StoreName:       sName,
			Stock22_12:      round2(s2212),
			Stock23_06:      round2(s2306),
			Stock23_12:      round2(s2312),
			Stock24_03:      round2(s2403),
			Stock24_06:      round2(s2406),
			Karcher24_03:    round2(k2403),
			KarcherSharePct: round2(kShare),
			StockGrowthPct:  round2(pctChange(s2403, s2212)),
			TotalSales15m:   round2(ca.total15m),
			SalesSharePct:   round2(salesShare),
			SalesQ1_2023:    round2(ca.q1_2023),
			SalesQ1_2024:    round2(ca.q1_2024),
			YoYGrowthQ1Pct:  round2(pctChange(ca.q1_2024, ca.q1_2023)),
			Avg6mSales:      round2(avg6),
			StockMonths:     round2(months),
			TurnoverStatus:  status,
		})

		rd.TotalStock22_12 += s2212
		rd.TotalStock23_06 += s2306
		rd.TotalStock23_12 += s2312
		rd.TotalStock24_03 += s2403
		rd.TotalStock24_06 += s2406
		rd.SalesQ1_2023 += ca.q1_2023
		rd.SalesQ1_2024 += ca.q1_2024
	}

	// Sort stores by TotalSales15m descending
	sort.Slice(rd.Stores, func(i, j int) bool {
		return rd.Stores[i].TotalSales15m > rd.Stores[j].TotalSales15m
	})

	rd.TotalStock22_12 = round2(rd.TotalStock22_12)
	rd.TotalStock23_06 = round2(rd.TotalStock23_06)
	rd.TotalStock23_12 = round2(rd.TotalStock23_12)
	rd.TotalStock24_03 = round2(rd.TotalStock24_03)
	rd.TotalStock24_06 = round2(rd.TotalStock24_06)
	rd.SalesQ1_2023 = round2(rd.SalesQ1_2023)
	rd.SalesQ1_2024 = round2(rd.SalesQ1_2024)
	rd.YoYGrowthQ1Pct = round2(pctChange(rd.SalesQ1_2024, rd.SalesQ1_2023))
	if totalAvg6m > 0 {
		rd.AvgStockMonths = round2(rd.TotalStock24_03 / totalAvg6m)
	}

	rep.Retail = rd
}

func extractCityName(s string) string {
	fields := strings.Fields(s)
	if len(fields) > 0 {
		return fields[0]
	}
	return s
}

func normalizeRetailPeriod(per string, row int) string {
	per = strings.ReplaceAll(per, "Сентярь", "Сентябрь")
	if !strings.Contains(per, "2023") && !strings.Contains(per, "2024") {
		if row <= 33 {
			per += " 2023"
		} else {
			per += " 2024"
		}
	}
	return per
}

func parseGroupsAndRF(wb *xlsx.Workbook, rep *DashboardReport) {
	var gd GroupsData
	if sh := wb.Sheet("Группы"); sh != nil {
		for r := 2; r <= 23 && r <= sh.MaxRow; r++ {
			sup := cleanText(sh.String(r, 2))
			if sup == "" || strings.HasPrefix(strings.ToUpper(sup), "ИТОГО") {
				continue
			}
			v23, ok23 := sh.FloatOk(r, 3)
			v24, ok24 := sh.FloatOk(r, 4)
			if !ok23 && !ok24 {
				continue
			}
			gd.Items = append(gd.Items, GroupStockRow{
				Supplier:  sup,
				Stock2023: round2(v23),
				Stock2024: round2(v24),
				Delta:     round2(v24 - v23),
				DeltaPct:  round2(pctChange(v24, v23)),
			})
			gd.TrackedStart += v23
			gd.TrackedEnd += v24
		}
		gd.TrackedStart = round2(gd.TrackedStart)
		gd.TrackedEnd = round2(gd.TrackedEnd)
		gd.TrackedDelta = round2(gd.TrackedEnd - gd.TrackedStart)
		gd.TrackedGrowthPct = round2(pctChange(gd.TrackedEnd, gd.TrackedStart))
		gd.CompanyTotalEnd = sh.Float(26, 4)
		if gd.CompanyTotalEnd == 0 && len(rep.SalesInventory.StockCheckpoints) > 0 {
			gd.CompanyTotalEnd = rep.SalesInventory.StockCheckpoints[len(rep.SalesInventory.StockCheckpoints)-1].ValueBYN
		}
		if gd.CompanyTotalEnd > 0 {
			gd.CoverageEndPct = round2(gd.TrackedEnd / gd.CompanyTotalEnd * 100.0)
		}

		// Sort descending by Stock2024 for ABC Pareto classification
		sort.Slice(gd.Items, func(i, j int) bool {
			return gd.Items[i].Stock2024 > gd.Items[j].Stock2024
		})
		cum := 0.0
		for i := range gd.Items {
			share := 0.0
			if gd.TrackedEnd > 0 {
				share = gd.Items[i].Stock2024 / gd.TrackedEnd * 100.0
			}
			prevCum := cum
			cum += share
			abc := "C"
			if prevCum < 80.0 {
				abc = "A"
			} else if prevCum < 95.0 {
				abc = "B"
			}
			gd.Items[i].SharePct = round2(share)
			gd.Items[i].CumShare = round2(cum)
			gd.Items[i].ABCClass = abc
		}
	}

	if srf := wb.Sheet("РФ"); srf != nil {
		for r := 3; r <= srf.MaxRow; r++ {
			sup := cleanText(srf.String(r, 1))
			if sup == "" || strings.HasPrefix(strings.ToUpper(sup), "ИТОГО") {
				continue
			}
			v1 := srf.Float(r, 2)
			v2 := srf.Float(r, 3)
			if v1 == 0 && v2 == 0 {
				continue
			}
			gd.RFSupplierStocks = append(gd.RFSupplierStocks, RFSupplierItem{
				Supplier:  sup,
				Stock2312: round2(v1),
				Stock2403: round2(v2),
				Delta:     round2(v2 - v1),
				DeltaPct:  round2(pctChange(v2, v1)),
			})
		}
		sort.Slice(gd.RFSupplierStocks, func(i, j int) bool {
			return gd.RFSupplierStocks[i].Stock2403 > gd.RFSupplierStocks[j].Stock2403
		})
	}

	rep.Groups = gd
}

func parseIntercompany(wb *xlsx.Workbook, rep *DashboardReport) {
	var ic IntercompanyData
	if sh := wb.Sheet("СМГ для ЧСТ"); sh != nil {
		for r := 2; r <= sh.MaxRow; r++ {
			sup := cleanText(sh.String(r, 2))
			if sup == "" || strings.HasPrefix(strings.ToUpper(sup), "ИТОГО") {
				break
			}
			q4 := sh.Float(r, 3)
			q1 := sh.Float(r, 4)
			ic.Items = append(ic.Items, IntercompanyRow{
				Supplier: sup,
				Q4_2023:  round2(q4),
				Q1_2024:  round2(q1),
				Delta:    round2(q1 - q4),
				DeltaPct: round2(pctChange(q1, q4)),
			})
			ic.TotalQ4_23 += q4
			ic.TotalQ1_24 += q1
		}
		ic.TotalQ4_23 = round2(ic.TotalQ4_23)
		ic.TotalQ1_24 = round2(ic.TotalQ1_24)
		ic.TotalDelta = round2(ic.TotalQ1_24 - ic.TotalQ4_23)
		ic.GrowthPct = round2(pctChange(ic.TotalQ1_24, ic.TotalQ4_23))

		sort.Slice(ic.Items, func(i, j int) bool {
			return ic.Items[i].Q1_2024 > ic.Items[j].Q1_2024
		})
		cum := 0.0
		for i := range ic.Items {
			share := 0.0
			if ic.TotalQ1_24 > 0 {
				share = ic.Items[i].Q1_2024 / ic.TotalQ1_24 * 100.0
			}
			prev := cum
			cum += share
			abc := "C"
			if prev < 80.0 {
				abc = "A"
			} else if prev < 95.0 {
				abc = "B"
			}
			ic.Items[i].ShareQ1 = round2(share)
			ic.Items[i].CumShare = round2(cum)
			ic.Items[i].ABCClass = abc
			if i == 3 {
				ic.Top4Share = round2(cum)
			}
		}
	}

	if sc := wb.Sheet("Поставщики Китай"); sc != nil {
		for r := 2; r <= sc.MaxRow; r++ {
			sup := cleanText(sc.String(r, 2))
			if sup == "" || strings.HasPrefix(strings.ToUpper(sup), "ИТОГО") {
				break
			}
			q4 := sc.Float(r, 3)
			q1 := sc.Float(r, 4)
			ic.ChinaItems = append(ic.ChinaItems, IntercompanyRow{
				Supplier: sup,
				Q4_2023:  round2(q4),
				Q1_2024:  round2(q1),
				Delta:    round2(q1 - q4),
				DeltaPct: round2(pctChange(q1, q4)),
			})
			ic.ChinaQ4_23 += q4
			ic.ChinaQ1_24 += q1
		}
		ic.ChinaQ4_23 = round2(ic.ChinaQ4_23)
		ic.ChinaQ1_24 = round2(ic.ChinaQ1_24)
		ic.ChinaDelta = round2(ic.ChinaQ1_24 - ic.ChinaQ4_23)
		ic.ChinaPct = round2(pctChange(ic.ChinaQ1_24, ic.ChinaQ4_23))
		for i := range ic.ChinaItems {
			if ic.ChinaQ1_24 > 0 {
				ic.ChinaItems[i].ShareQ1 = round2(ic.ChinaItems[i].Q1_2024 / ic.ChinaQ1_24 * 100.0)
			}
		}
	}

	rep.Intercompany = ic
}

func parsePrepayments(wb *xlsx.Workbook, rep *DashboardReport) {
	sh := wb.Sheet("Предварительные оплаты")
	if sh == nil {
		return
	}
	var pd PrepaymentsData

	// Parse FX rates from cols I & J (rows 3..8)
	rateMap := map[string]float64{
		"USD": 3.2047,
		"EUR": 3.4725,
		"RUB": 0.036462,
		"CNY": 0.43975,
		"PLN": 0.81388,
	}
	for r := 2; r <= 10 && r <= sh.MaxRow; r++ {
		rateVal, ok := sh.FloatOk(r, 9)
		lbl := cleanText(sh.String(r, 10))
		if !ok || lbl == "" {
			continue
		}
		code := mapCurrencyCode(lbl)
		if code != "" {
			rateMap[code] = rateVal
			pd.Rates = append(pd.Rates, FXRateItem{
				Code:     code,
				Label:    lbl,
				UnitRate: rateVal,
			})
		}
	}

	section := "paid"
	expByCode := make(map[string]*CurrencyBucket)
	codeOrder := []string{"EUR", "CNY", "RUB", "PLN", "USD"}

	for r := 1; r <= sh.MaxRow; r++ {
		colA := cleanText(sh.String(r, 1))
		lowA := strings.ToLower(colA)
		switch {
		case strings.Contains(lowA, "уже оплаченные"):
			section = "paid"
			continue
		case strings.Contains(lowA, "необходимо оплатить"):
			section = "unpaid"
			continue
		case strings.Contains(lowA, "задолженность керхер"):
			if _, ok := sh.FloatOk(r, 2); !ok {
				section = "debt"
				continue
			}
		case strings.Contains(lowA, "уже оплачено + таможня") || strings.HasPrefix(lowA, "итого"):
			section = "summary"
			continue
		}
		if section == "summary" || colA == "" || lowA == "поставщик" {
			continue
		}

		amtOrig, okAmt := sh.FloatOk(r, 2)
		if !okAmt || amtOrig == 0 {
			continue
		}
		currRaw := cleanText(sh.String(r, 3))
		code := mapCurrencyCode(currRaw)
		custOrig := sh.Float(r, 4)
		purchBYN := sh.Float(r, 5)
		custBYN := sh.Float(r, 6)
		if purchBYN == 0 && code != "" {
			purchBYN = amtOrig * rateMap[code]
		}
		custRate := 0.0
		if amtOrig > 0 && custOrig > 0 {
			custRate = custOrig / amtOrig
		}

		if section == "debt" {
			pd.KarcherRFDebtRUB = round2(amtOrig)
			pd.KarcherRFDebtBYN = round2(purchBYN)
			addCurrencyExposure(expByCode, "RUB", "Российский рубль (RUB)", amtOrig, purchBYN)
			continue
		}

		item := PrepaymentItem{
			Status:       section,
			Supplier:     colA,
			AmountOrig:   round2(amtOrig),
			Currency:     currRaw,
			CurrencyCode: code,
			CustomsOrig:  round2(custOrig),
			CustomsRate:  math.Round(custRate*10000) / 10000,
			PurchaseBYN:  round2(purchBYN),
			CustomsBYN:   round2(custBYN),
			TotalBYN:     round2(purchBYN + custBYN),
		}
		if section == "paid" {
			pd.PaidOrders = append(pd.PaidOrders, item)
			pd.PaidPurchaseBYN += purchBYN
			pd.PaidCustomsBYN += custBYN
		} else if section == "unpaid" {
			pd.UnpaidOrders = append(pd.UnpaidOrders, item)
			pd.UnpaidPurchaseBYN += purchBYN
			pd.UnpaidCustomsBYN += custBYN
		}
		addCurrencyExposure(expByCode, code, currencyTitle(code), amtOrig+custOrig, purchBYN+custBYN)
	}

	pd.PaidPurchaseBYN = round2(pd.PaidPurchaseBYN)
	pd.PaidCustomsBYN = round2(pd.PaidCustomsBYN)
	pd.PaidTotalBYN = round2(pd.PaidPurchaseBYN + pd.PaidCustomsBYN)
	pd.UnpaidPurchaseBYN = round2(pd.UnpaidPurchaseBYN)
	pd.UnpaidCustomsBYN = round2(pd.UnpaidCustomsBYN)
	pd.UnpaidTotalBYN = round2(pd.UnpaidPurchaseBYN + pd.UnpaidCustomsBYN)
	pd.GrandTotalBYN = round2(pd.PaidTotalBYN + pd.UnpaidTotalBYN + pd.KarcherRFDebtBYN)

	for _, c := range codeOrder {
		if b, ok := expByCode[c]; ok && b.TotalBYN > 0 {
			b.AmountOrig = round2(b.AmountOrig)
			b.TotalBYN = round2(b.TotalBYN)
			if pd.GrandTotalBYN > 0 {
				b.SharePct = round2(b.TotalBYN / pd.GrandTotalBYN * 100.0)
			}
			pd.CurrencyExposure = append(pd.CurrencyExposure, *b)
		}
	}
	sort.Slice(pd.CurrencyExposure, func(i, j int) bool {
		return pd.CurrencyExposure[i].TotalBYN > pd.CurrencyExposure[j].TotalBYN
	})

	rep.Prepayments = pd
}

func addCurrencyExposure(m map[string]*CurrencyBucket, code, name string, orig, byn float64) {
	if code == "" {
		code = "OTHER"
	}
	b := m[code]
	if b == nil {
		b = &CurrencyBucket{CurrencyCode: code, CurrencyName: name}
		m[code] = b
	}
	b.AmountOrig += orig
	b.TotalBYN += byn
}

func mapCurrencyCode(s string) string {
	low := strings.ToLower(s)
	switch {
	case strings.Contains(low, "евро") || strings.Contains(low, "eur"):
		return "EUR"
	case strings.Contains(low, "юан") || strings.Contains(low, "cny"):
		return "CNY"
	case strings.Contains(low, "росс") || strings.Contains(low, "rub"):
		return "RUB"
	case strings.Contains(low, "злот") || strings.Contains(low, "pln"):
		return "PLN"
	case strings.Contains(low, "долл") || strings.Contains(low, "usd"):
		return "USD"
	default:
		return ""
	}
}

func currencyTitle(code string) string {
	switch code {
	case "EUR":
		return "Евро (EUR)"
	case "CNY":
		return "Китайский юань (CNY)"
	case "RUB":
		return "Российский рубль (RUB)"
	case "PLN":
		return "Польский злотый (PLN)"
	case "USD":
		return "Доллар США (USD)"
	default:
		return code
	}
}

func parseRFProcurement(wb *xlsx.Workbook, rep *DashboardReport) {
	var rfp RFProcurementData

	// 1. Comparison Q4 2023 vs Q1 2024
	if sc := wb.Sheet("4-ый квартал 2023 (сравнение)"); sc != nil {
		for r := 9; r <= 32 && r <= sc.MaxRow; r++ {
			sup := cleanText(sc.String(r, 1))
			if sup == "" {
				continue
			}
			if strings.HasPrefix(strings.ToUpper(sup), "ИТОГО") {
				rfp.ExternalQ4_2023 = round2(sc.Float(r, 2))
				rfp.ExternalQ1_2024 = round2(sc.Float(r, 3))
				rfp.ExternalDelta = round2(rfp.ExternalQ1_2024 - rfp.ExternalQ4_2023)
				rfp.ExternalGrowthPct = round2(pctChange(rfp.ExternalQ1_2024, rfp.ExternalQ4_2023))
				break
			}
			q4 := sc.Float(r, 2)
			q1 := sc.Float(r, 3)
			cat := cleanText(sc.String(r, 11))
			if cat == "" {
				if sup == "Разовые закупки" || sup == "Закупки сервис" {
					cat = "Сводная группа"
				} else {
					cat = "Основные поставщики"
				}
			}
			hasPct := q4 > 0
			rfp.ComparisonRows = append(rfp.ComparisonRows, RFComparisonRow{
				Supplier: sup,
				Category: cat,
				Q4_2023:  round2(q4),
				Q1_2024:  round2(q1),
				Delta:    round2(q1 - q4),
				DeltaPct: round2(pctChange(q1, q4)),
				HasPct:   hasPct,
			})
		}
	}

	// 2. Full 1C report sheet "Закупки РФ-РФ"
	if sz := wb.Sheet("Закупки РФ-РФ"); sz != nil {
		rfp.Period1C = strings.TrimPrefix(cleanText(sz.String(2, 3)), "Период:")
		rfp.Period1C = strings.TrimSpace(rfp.Period1C)

		var curSup *RFSupplier1CRow
		var curDocNum, curDocDate, curMonth string

		monthMap := map[string]*RFMonthly1CPoint{
			"01": {Month: "Январь 2024"},
			"02": {Month: "Февраль 2024"},
			"03": {Month: "Март 2024"},
		}
		type skuAgg struct {
			sku      string
			supplier string
			category string
			qty      float64
			sum      float64
			orders   int
		}
		skuMap := make(map[string]*skuAgg)

		for r := 11; r <= sz.MaxRow; r++ {
			cellA := sz.Cell(r, 1)
			txtA := cleanText(cellA.Text)
			if txtA == "" {
				continue
			}
			if strings.EqualFold(txtA, "Итого") {
				rfp.Total1CQty = round2(sz.Float(r, 6))
				rfp.Total1CNoVAT = round2(sz.Float(r, 7))
				rfp.Total1CVAT = round2(sz.Float(r, 8))
				rfp.Total1CWithVAT = round2(sz.Float(r, 9))
				break
			}

			switch cellA.Indent {
			case 0:
				if curSup != nil {
					rfp.Suppliers1C = append(rfp.Suppliers1C, *curSup)
				}
				rawTag := cleanText(sz.String(r, 10))
				cat := classify1CSupplierTag(rawTag)
				curSup = &RFSupplier1CRow{
					Supplier:   txtA,
					Category:   cat,
					RawTag:     rawTag,
					Qty:        round2(sz.Float(r, 6)),
					SumNoVAT:   round2(sz.Float(r, 7)),
					SumVAT:     round2(sz.Float(r, 8)),
					SumWithVAT: round2(sz.Float(r, 9)),
				}
			case 2:
				rfp.DocumentCount++
				if curSup != nil {
					curSup.DocCount++
				}
				curDocDate = ""
				curDocNum = txtA
				curMonth = "1 кв. 2024"
				if m := docNumRegex.FindStringSubmatch(txtA); len(m) > 1 {
					curDocNum = m[1]
				}
				var mmKey string
				if dm := docDateRegex.FindStringSubmatch(txtA); len(dm) == 4 {
					curDocDate = dm[1] + "." + dm[2] + "." + dm[3]
					mmKey = dm[2]
					switch mmKey {
					case "01":
						curMonth = "Январь 2024"
					case "02":
						curMonth = "Февраль 2024"
					case "03":
						curMonth = "Март 2024"
					}
				}
				docSum := sz.Float(r, 9)
				if mp, ok := monthMap[mmKey]; ok && curSup != nil {
					mp.DocCount++
					mp.TotalRUB += docSum
					if curSup.RawTag == "-" {
						mp.InternalRUB += docSum
					} else {
						mp.ExternalRUB += docSum
					}
				}
			default: // indent >= 4 -> SKU item
				rfp.LineItemCount++
				if curSup != nil {
					curSup.ItemCount++
				}
				qty := sz.Float(r, 6)
				sNoVAT := sz.Float(r, 7)
				sVAT := sz.Float(r, 8)
				sWithVAT := sz.Float(r, 9)
				supName := ""
				supCat := ""
				if curSup != nil {
					supName = curSup.Supplier
					supCat = curSup.Category
				}
				rfp.LineItems = append(rfp.LineItems, RFLineItem{
					Supplier:   supName,
					Category:   supCat,
					DocNumber:  curDocNum,
					DocDate:    curDocDate,
					Month:      curMonth,
					SKU:        txtA,
					VATRate:    cleanText(sz.String(r, 5)),
					Qty:        round2(qty),
					SumNoVAT:   round2(sNoVAT),
					SumVAT:     round2(sVAT),
					SumWithVAT: round2(sWithVAT),
				})
				key := supName + "||" + txtA
				sa := skuMap[key]
				if sa == nil {
					sa = &skuAgg{sku: txtA, supplier: supName, category: supCat}
					skuMap[key] = sa
				}
				sa.qty += qty
				sa.sum += sWithVAT
				sa.orders++
			}
		}
		if curSup != nil {
			rfp.Suppliers1C = append(rfp.Suppliers1C, *curSup)
		}

		if rfp.Total1CWithVAT == 0 {
			for _, s := range rfp.Suppliers1C {
				rfp.Total1CQty += s.Qty
				rfp.Total1CNoVAT += s.SumNoVAT
				rfp.Total1CVAT += s.SumVAT
				rfp.Total1CWithVAT += s.SumWithVAT
			}
		}

		sort.Slice(rfp.Suppliers1C, func(i, j int) bool {
			return rfp.Suppliers1C[i].SumWithVAT > rfp.Suppliers1C[j].SumWithVAT
		})
		for i := range rfp.Suppliers1C {
			if rfp.Total1CWithVAT > 0 {
				rfp.Suppliers1C[i].SharePct = round2(rfp.Suppliers1C[i].SumWithVAT / rfp.Total1CWithVAT * 100.0)
			}
		}

		for _, k := range []string{"01", "02", "03"} {
			mp := monthMap[k]
			mp.ExternalRUB = round2(mp.ExternalRUB)
			mp.InternalRUB = round2(mp.InternalRUB)
			mp.TotalRUB = round2(mp.TotalRUB)
			rfp.Monthly1C = append(rfp.Monthly1C, *mp)
		}

		var skus []RFSKURow
		for _, sa := range skuMap {
			skus = append(skus, RFSKURow{
				SKU:        sa.sku,
				Supplier:   sa.supplier,
				Category:   sa.category,
				Qty:        round2(sa.qty),
				SumWithVAT: round2(sa.sum),
				OrderCount: sa.orders,
			})
		}
		sort.Slice(skus, func(i, j int) bool {
			return skus[i].SumWithVAT > skus[j].SumWithVAT
		})
		if len(skus) > 40 {
			skus = skus[:40]
		}
		rfp.TopSKUs = skus
	}

	rep.RFProcurement = rfp
}

func classify1CSupplierTag(tag string) string {
	switch tag {
	case "+":
		return "Основные внешние (+)"
	case "-":
		return "Керхер / Внутригрупповые (-)"
	case "Закупки сервис":
		return "Закупки сервис"
	case "Разовые закупки":
		return "Разовые закупки"
	default:
		if tag == "" {
			return "Прочие"
		}
		return tag
	}
}

func buildOverviewKPIs(rep *DashboardReport) {
	var k OverviewKPIs

	for _, m := range rep.SalesInventory.MonthlySales {
		if m.Period == "2П 2023" {
			k.SalesH2_2023 += m.TotalSales
		} else if m.Period == "1П 2024" {
			k.SalesH1_2024 += m.TotalSales
		}
		if m.TotalSales > k.PeakSalesValue {
			k.PeakSalesValue = m.TotalSales
			k.PeakSalesMonth = m.FullLabel
		}
	}
	k.SalesH2_2023 = round2(k.SalesH2_2023)
	k.SalesH1_2024 = round2(k.SalesH1_2024)
	k.SalesDeltaAbs = round2(k.SalesH1_2024 - k.SalesH2_2023)
	k.SalesGrowthPct = round2(pctChange(k.SalesH1_2024, k.SalesH2_2023))
	if k.SalesH2_2023 > 0 {
		k.AvgMonthlySalesH2_23 = round2(k.SalesH2_2023 / 6.0)
	}
	if k.SalesH1_2024 > 0 {
		k.AvgMonthlySalesH1_24 = round2(k.SalesH1_2024 / 6.0)
	}

	cps := rep.SalesInventory.StockCheckpoints
	if len(cps) >= 4 {
		k.StockEnd2022 = cps[0].ValueBYN
		k.StockMid2023 = cps[1].ValueBYN
		k.StockEnd2023 = cps[2].ValueBYN
		k.StockMid2024 = cps[3].ValueBYN
	} else if len(cps) >= 2 {
		k.StockEnd2023 = cps[len(cps)-2].ValueBYN
		k.StockMid2024 = cps[len(cps)-1].ValueBYN
	}
	k.StockDeltaH1_2024 = round2(k.StockMid2024 - k.StockEnd2023)
	k.StockGrowthH1_2024Pct = round2(pctChange(k.StockMid2024, k.StockEnd2023))
	if k.StockEnd2022 > 0 {
		k.StockGrowth18mPct = round2(pctChange(k.StockMid2024, k.StockEnd2022))
	}
	if k.AvgMonthlySalesH2_23 > 0 {
		k.StockMonthsEnd2023 = round2(k.StockEnd2023 / k.AvgMonthlySalesH2_23)
	}
	if k.AvgMonthlySalesH1_24 > 0 {
		k.StockMonthsMid2024 = round2(k.StockMid2024 / k.AvgMonthlySalesH1_24)
	}
	avgStockH1 := (k.StockEnd2023 + k.StockMid2024) / 2.0
	if avgStockH1 > 0 && k.SalesH1_2024 > 0 {
		k.AnnualTurnoverRatio = round2((k.SalesH1_2024 * 2.0) / avgStockH1)
	}

	k.KarcherSalesH1_2024 = rep.Karcher.SalesH1_2024
	if k.SalesH1_2024 > 0 {
		k.KarcherShareH1_2024 = round2(k.KarcherSalesH1_2024 / k.SalesH1_2024 * 100.0)
	}
	k.KarcherYoYGrowthPct = round2(pctChange(rep.Karcher.SalesH1_2024, rep.Karcher.SalesH1_2023))

	k.RetailSalesQ1_2024 = rep.Retail.SalesQ1_2024
	k.RetailYoYGrowthPct = rep.Retail.YoYGrowthQ1Pct

	k.SMGToChST_Q1_2024 = rep.Intercompany.TotalQ1_24
	k.SMGToChSTGrowthPct = rep.Intercompany.GrowthPct

	k.TotalCommitmentsBYN = rep.Prepayments.GrandTotalBYN
	k.RFExternalQ1_2024RUB = rep.RFProcurement.ExternalQ1_2024
	k.RFExternalGrowthPct = rep.RFProcurement.ExternalGrowthPct
	k.RFTotal1C_Q1_2024RUB = rep.RFProcurement.Total1CWithVAT

	rep.KPIs = k
}

func buildExecutiveInsights(rep *DashboardReport) {
	k := rep.KPIs
	var ins []InsightItem

	if k.SalesH1_2024 > 0 {
		ins = append(ins, InsightItem{
			Severity: "positive",
			Category: "Реализация 1П 2024",
			Title:    "Уверенный рост реализации по компании",
			Metric:   fmt.Sprintf("+%.1f%% (+%.0f тыс. BYN)", k.SalesGrowthPct, k.SalesDeltaAbs/1000.0),
			Detail: fmt.Sprintf(
				"Реализация по себестоимости в 1-м полугодии 2024 выросла до %.2f млн BYN (средняя %.0f тыс. BYN/мес против %.0f тыс. BYN/мес во 2П 2023). Пик достигнут в месяце %s (%.2f млн BYN).",
				k.SalesH1_2024/1e6, k.AvgMonthlySalesH1_24/1e3, k.AvgMonthlySalesH2_23/1e3, k.PeakSalesMonth, k.PeakSalesValue/1e6,
			),
		})
	}

	if k.StockGrowthH1_2024Pct > k.SalesGrowthPct && k.StockMid2024 > 0 {
		ins = append(ins, InsightItem{
			Severity: "warning",
			Category: "Оборачиваемость и запасы",
			Title:    "Темп роста складских запасов опережает темп роста продаж",
			Metric:   fmt.Sprintf("Склад +%.1f%% vs Продажи +%.1f%%", k.StockGrowthH1_2024Pct, k.SalesGrowthPct),
			Detail: fmt.Sprintf(
				"Товарные остатки выросли с %.2f до %.2f млн BYN (+%.0f тыс. BYN). Покрытие продаж запасами увеличилось с %.2f до %.2f мес. (с конца 2022 года склад вырос на +%.1f%%).",
				k.StockEnd2023/1e6, k.StockMid2024/1e6, k.StockDeltaH1_2024/1e3, k.StockMonthsEnd2023, k.StockMonthsMid2024, k.StockGrowth18mPct,
			),
		})
	}

	if rep.Karcher.SalesH1_2024 > 0 {
		ins = append(ins, InsightItem{
			Severity: "info",
			Category: "Направление Керхер",
			Title:    "Флагман продаж (38.7% оборота) с профицитом закупок",
			Metric:   fmt.Sprintf("YoY +%.1f%% | Накопление +%.0f тыс. BYN", k.KarcherYoYGrowthPct, rep.Karcher.NetBuildH1_2024/1e3),
			Detail: fmt.Sprintf(
				"Продажи Керхер в 1П 2024 составили %.2f млн BYN (+%.1f%% к 1П 2023), но закупки за 1П 2024 достигли %.2f млн BYN. Склад КЕРХЕР H&G вырос на +62.6%% (до %.2f млн BYN), КЕРХЕР проф — до %.2f млн BYN.",
				rep.Karcher.SalesH1_2024/1e6, k.KarcherYoYGrowthPct, rep.Karcher.PurchH1_2024/1e6, rep.Karcher.StockHG_Mid2024/1e6, rep.Karcher.StockProfMid2024/1e6,
			),
		})
	}

	if len(rep.Retail.Stores) >= 2 {
		best := rep.Retail.Stores[0]
		var slowest RetailStoreSummary
		for _, st := range rep.Retail.Stores {
			if st.StockMonths > slowest.StockMonths {
				slowest = st
			}
		}
		ins = append(ins, InsightItem{
			Severity: "warning",
			Category: "Розничная сеть",
			Title:    fmt.Sprintf("Дисбаланс оборачиваемости: лидер %s vs затоваривание в г. %s", best.City, slowest.City),
			Metric:   fmt.Sprintf("%s: %.2f мес. | %s: %.2f мес.", best.City, best.StockMonths, slowest.City, slowest.StockMonths),
			Detail: fmt.Sprintf(
				"Розничные продажи в 1 кв. 2024 выросли на +%.1f%% YoY. Магазин %s генерирует %.0f тыс. BYN/мес (оборачиваемость %.2f мес.), тогда как %s имеет запас на %.2f мес. — рекомендуется ротация стока между филиалами.",
				rep.Retail.YoYGrowthQ1Pct, best.City, best.Avg6mSales/1e3, best.StockMonths, slowest.City, slowest.StockMonths,
			),
		})
	}

	if rep.Intercompany.TotalQ1_24 > 0 {
		ins = append(ins, InsightItem{
			Severity: "positive",
			Category: "Экспорт СМГ → ЧСТ (РФ)",
			Title:    "Ускорение поставок на РФ и высокая маржинальная доля собственного производства",
			Metric:   fmt.Sprintf("+%.1f%% к 4 кв. 2023 (%.0f тыс. BYN)", rep.Intercompany.GrowthPct, rep.Intercompany.TotalQ1_24/1e3),
			Detail: fmt.Sprintf(
				"Отгрузки СМГ для ЧСТ выросли с %.0f до %.0f тыс. BYN. Топ-4 товарные группы формируют %.1f%% объёма (собственные системы и запасные части — 66.4%% от всех поставок).",
				rep.Intercompany.TotalQ4_23/1e3, rep.Intercompany.TotalQ1_24/1e3, rep.Intercompany.Top4Share,
			),
		})
	}

	if rep.Prepayments.GrandTotalBYN > 0 {
		ins = append(ins, InsightItem{
			Severity: "critical",
			Category: "Финансы и Валютные обязательства",
			Title:    "Существенная финансовая нагрузка по заказам в пути и расчётам с поставщиками",
			Metric:   fmt.Sprintf("%.2f млн BYN обязательств", rep.Prepayments.GrandTotalBYN/1e6),
			Detail: fmt.Sprintf(
				"Уже оплачено с учётом таможни %.2f млн BYN, к оплате по заказам ещё %.2f млн BYN + задолженность Керхер РФ %.0f тыс. BYN (13.0 млн RUB). По приходу товаров склад дополнительно вырастет.",
				rep.Prepayments.PaidTotalBYN/1e6, rep.Prepayments.UnpaidTotalBYN/1e6, rep.Prepayments.KarcherRFDebtBYN/1e3,
			),
		})
	}

	rep.Insights = ins
}

func buildRawAndGenericSheets(wb *xlsx.Workbook, rep *DashboardReport) {
	const maxRawRows = 500
	for _, sh := range wb.Sheets {
		limit := sh.MaxRow
		truncated := false
		if limit > maxRawRows {
			limit = maxRawRows
			truncated = true
		}
		maxCol := sh.MaxCol
		if maxCol > 20 {
			maxCol = 20
		}

		rv := RawSheetView{
			Name:      sh.Name,
			MaxRow:    sh.MaxRow,
			MaxCol:    sh.MaxCol,
			Truncated: truncated,
		}
		for r := 1; r <= limit; r++ {
			rowCells := make([]RawCellView, maxCol)
			hasAny := false
			for c := 1; c <= maxCol; c++ {
				cell := sh.Cell(r, c)
				txt := cleanText(cell.Text)
				if cell.HasFloat && !cell.IsDate {
					if cell.IsPercent {
						txt = fmt.Sprintf("%.2f%%", cell.FloatVal*100.0)
					} else {
						txt = formatNumberRU(cell.FloatVal)
					}
				}
				if txt != "" || cell.Formula != "" {
					hasAny = true
				}
				rowCells[c-1] = RawCellView{
					Text:    txt,
					Formula: cell.Formula,
					IsNum:   cell.HasFloat && !cell.IsDate,
					IsPct:   cell.IsPercent,
					Indent:  cell.Indent,
				}
			}
			if hasAny {
				rv.Rows = append(rv.Rows, rowCells)
			} else if len(rv.Rows) > 0 && r < limit {
				rv.Rows = append(rv.Rows, make([]RawCellView, maxCol))
			}
		}
		rep.RawSheets = append(rep.RawSheets, rv)

		// Also compute GenericSheetStat for arbitrary sheets
		if gs, ok := analyzeGenericSheet(sh); ok {
			rep.GenericTables = append(rep.GenericTables, gs)
		}
	}
}

func formatNumberRU(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	rounded := math.Round(v*100) / 100
	isInt := math.Abs(rounded-math.Round(rounded)) < 1e-6
	var intPart int64
	var decPart string
	if isInt {
		intPart = int64(math.Round(rounded))
	} else {
		s := strconv.FormatFloat(rounded, 'f', 2, 64)
		parts := strings.Split(s, ".")
		intPart, _ = strconv.ParseInt(parts[0], 10, 64)
		decPart = "," + parts[1]
	}
	intStr := strconv.FormatInt(intPart, 10)
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	for i, ch := range intStr {
		if i > 0 && (len(intStr)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(ch)
	}
	b.WriteString(decPart)
	return b.String()
}

func analyzeGenericSheet(sh *xlsx.Sheet) (GenericSheetStat, bool) {
	if sh.MaxRow < 2 || sh.MaxCol < 2 {
		return GenericSheetStat{}, false
	}
	// Find header row in first 10 rows with at least 2 text cells
	headerRow := 1
	bestHeaders := 0
	for r := 1; r <= 10 && r <= sh.MaxRow; r++ {
		cnt := 0
		for c := 1; c <= sh.MaxCol && c <= 20; c++ {
			cell := sh.Cell(r, c)
			if strings.TrimSpace(cell.Text) != "" && (!cell.HasFloat || cell.IsDate) {
				cnt++
			}
		}
		if cnt > bestHeaders {
			bestHeaders = cnt
			headerRow = r
		}
	}

	gs := GenericSheetStat{
		SheetName: sh.Name,
		RowCount:  sh.MaxRow,
		ColCount:  sh.MaxCol,
	}

	labelCol := 1
	for c := 1; c <= sh.MaxCol && c <= 5; c++ {
		if cleanText(sh.String(headerRow+1, c)) != "" && !sh.Cell(headerRow+1, c).HasFloat {
			labelCol = c
			break
		}
	}

	firstMetricCol := 0
	for c := 1; c <= sh.MaxCol && c <= 20; c++ {
		if c == labelCol {
			continue
		}
		hdr := cleanText(sh.String(headerRow, c))
		if hdr == "" {
			hdr = "Колонка " + xlsx.ColName(c)
		}
		var sum, minV, maxV float64
		cnt := 0
		for r := headerRow + 1; r <= sh.MaxRow; r++ {
			lbl := strings.ToUpper(cleanText(sh.String(r, labelCol)))
			if strings.HasPrefix(lbl, "ИТОГО") || lbl == "ИТОГ" {
				continue
			}
			cell := sh.Cell(r, c)
			if cell.HasFloat && !cell.IsDate && !cell.IsPercent {
				v := cell.FloatVal
				if cnt == 0 || v < minV {
					minV = v
				}
				if cnt == 0 || v > maxV {
					maxV = v
				}
				sum += v
				cnt++
			}
		}
		if cnt >= 2 {
			if firstMetricCol == 0 {
				firstMetricCol = c
			}
			gs.MetricCols = append(gs.MetricCols, GenericMetricStat{
				Header: hdr,
				Count:  cnt,
				Sum:    round2(sum),
				Avg:    round2(sum / float64(cnt)),
				Min:    round2(minV),
				Max:    round2(maxV),
			})
		}
	}

	if firstMetricCol > 0 {
		for r := headerRow + 1; r <= sh.MaxRow; r++ {
			lbl := cleanText(sh.String(r, labelCol))
			if lbl == "" || strings.HasPrefix(strings.ToUpper(lbl), "ИТОГО") || strings.EqualFold(lbl, "Итого") {
				continue
			}
			if v, ok := sh.FloatOk(r, firstMetricCol); ok && v > 0 {
				gs.TopPreview = append(gs.TopPreview, GenericLabelVal{
					Label: lbl,
					Value: round2(v),
				})
			}
		}
		sort.Slice(gs.TopPreview, func(i, j int) bool {
			return gs.TopPreview[i].Value > gs.TopPreview[j].Value
		})
		if len(gs.TopPreview) > 12 {
			gs.TopPreview = gs.TopPreview[:12]
		}
	}

	return gs, len(gs.MetricCols) > 0
}
