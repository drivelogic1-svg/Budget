package analytics

// DashboardReport is the complete analytical report produced from an Excel workbook.
type DashboardReport struct {
	FileName        string             `json:"fileName"`
	LoadedAt        string             `json:"loadedAt"`
	SheetCount      int                `json:"sheetCount"`
	SheetNames      []string           `json:"sheetNames"`
	IsBudgetDataset bool               `json:"isBudgetDataset"`
	KPIs            OverviewKPIs       `json:"kpis"`
	Insights        []InsightItem      `json:"insights"`
	SalesInventory  SalesInventoryData `json:"salesInventory"`
	Karcher         KarcherData        `json:"karcher"`
	Retail          RetailData         `json:"retail"`
	Warehouses      WarehousesData     `json:"warehouses"`
	Groups          GroupsData         `json:"groups"`
	Intercompany    IntercompanyData   `json:"intercompany"`
	Prepayments     PrepaymentsData    `json:"prepayments"`
	RFProcurement   RFProcurementData  `json:"rfProcurement"`
	GenericTables   []GenericSheetStat `json:"genericTables"`
	RawSheets       []RawSheetView     `json:"rawSheets"`
}

// OverviewKPIs summarizes top-level sales, inventory, turnover, and financial metrics.
type OverviewKPIs struct {
	SalesH2_2023          float64 `json:"salesH2_2023"`
	SalesH1_2024          float64 `json:"salesH1_2024"`
	SalesDeltaAbs         float64 `json:"salesDeltaAbs"`
	SalesGrowthPct        float64 `json:"salesGrowthPct"`
	AvgMonthlySalesH2_23  float64 `json:"avgMonthlySalesH2_23"`
	AvgMonthlySalesH1_24  float64 `json:"avgMonthlySalesH1_24"`
	PeakSalesMonth        string  `json:"peakSalesMonth"`
	PeakSalesValue        float64 `json:"peakSalesValue"`
	StockEnd2022          float64 `json:"stockEnd2022"`
	StockMid2023          float64 `json:"stockMid2023"`
	StockEnd2023          float64 `json:"stockEnd2023"`
	StockMid2024          float64 `json:"stockMid2024"`
	StockDeltaH1_2024     float64 `json:"stockDeltaH1_2024"`
	StockGrowthH1_2024Pct float64 `json:"stockGrowthH1_2024Pct"`
	StockGrowth18mPct     float64 `json:"stockGrowth18mPct"`
	StockMonthsEnd2023    float64 `json:"stockMonthsEnd2023"`
	StockMonthsMid2024    float64 `json:"stockMonthsMid2024"`
	AnnualTurnoverRatio   float64 `json:"annualTurnoverRatio"`
	KarcherSalesH1_2024   float64 `json:"karcherSalesH1_2024"`
	KarcherShareH1_2024   float64 `json:"karcherShareH1_2024"`
	KarcherYoYGrowthPct   float64 `json:"karcherYoYGrowthPct"`
	RetailSalesQ1_2024    float64 `json:"retailSalesQ1_2024"`
	RetailYoYGrowthPct    float64 `json:"retailYoYGrowthPct"`
	SMGToChST_Q1_2024     float64 `json:"smgToChstQ1_2024"`
	SMGToChSTGrowthPct    float64 `json:"smgToChstGrowthPct"`
	TotalCommitmentsBYN   float64 `json:"totalCommitmentsBYN"`
	RFExternalQ1_2024RUB  float64 `json:"rfExternalQ1_2024RUB"`
	RFExternalGrowthPct   float64 `json:"rfExternalGrowthPct"`
	RFTotal1C_Q1_2024RUB  float64 `json:"rfTotal1C_Q1_2024RUB"`
}

// InsightItem represents an automated analytical finding or recommendation.
type InsightItem struct {
	Severity string `json:"severity"` // "positive", "warning", "info", "critical"
	Category string `json:"category"`
	Title    string `json:"title"`
	Metric   string `json:"metric"`
	Detail   string `json:"detail"`
}

// SalesInventoryData captures company-wide stock checkpoints and 12-month sales dynamics.
type SalesInventoryData struct {
	StockCheckpoints []StockCheckpoint  `json:"stockCheckpoints"`
	MonthlySales     []MonthlySalePoint `json:"monthlySales"`
}

type StockCheckpoint struct {
	DateLabel string  `json:"dateLabel"`
	Label     string  `json:"label"`
	ValueBYN  float64 `json:"valueBYN"`
	DeltaPrev float64 `json:"deltaPrev"`
	DeltaPct  float64 `json:"deltaPct"`
}

type MonthlySalePoint struct {
	Period       string  `json:"period"` // "2П 2023" or "1П 2024"
	Month        string  `json:"month"`
	FullLabel    string  `json:"fullLabel"`
	TotalSales   float64 `json:"totalSales"`
	KarcherSales float64 `json:"karcherSales"`
	OtherSales   float64 `json:"otherSales"`
	KarcherShare float64 `json:"karcherShare"`
	MoMGrowthPct float64 `json:"momGrowthPct"`
}

// KarcherData captures 18-month sales vs purchases and H&G vs Prof segment analytics.
type KarcherData struct {
	Monthly          []KarcherMonthRow   `json:"monthly"`
	Segments2023     []KarcherSegmentRow `json:"segments2023"`
	TotalSales18m    float64             `json:"totalSales18m"`
	TotalPurch18m    float64             `json:"totalPurch18m"`
	TotalDiff18m     float64             `json:"totalDiff18m"`
	SalesH1_2023     float64             `json:"salesH1_2023"`
	SalesH2_2023     float64             `json:"salesH2_2023"`
	SalesH1_2024     float64             `json:"salesH1_2024"`
	PurchH1_2024     float64             `json:"purchH1_2024"`
	NetBuildH1_2024  float64             `json:"netBuildH1_2024"`
	StockHG_End2023  float64             `json:"stockHG_End2023"`
	StockHG_Mid2024  float64             `json:"stockHG_Mid2024"`
	StockProfEnd2023 float64             `json:"stockProfEnd2023"`
	StockProfMid2024 float64             `json:"stockProfMid2024"`
	SoldToKarcherRF  float64             `json:"soldToKarcherRF"`
	SoldRFNote       string              `json:"soldRFNote"`
}

type KarcherMonthRow struct {
	Month        string  `json:"month"`
	HalfYear     string  `json:"halfYear"`
	SalesBYN     float64 `json:"salesBYN"`
	PurchasesBYN float64 `json:"purchasesBYN"`
	DiffBYN      float64 `json:"diffBYN"` // Purchases - Sales
	CumDiffBYN   float64 `json:"cumDiffBYN"`
	PurchToSales float64 `json:"purchToSales"`
}

type KarcherSegmentRow struct {
	Month     string  `json:"month"`
	HalfYear  string  `json:"halfYear"`
	HG        float64 `json:"hg"`
	Prof      float64 `json:"prof"`
	Total     float64 `json:"total"`
	HGShare   float64 `json:"hgShare"`
	ProfShare float64 `json:"profShare"`
}

// RetailData captures the 4 regional retail stores' sales, stock, and turnover.
type RetailData struct {
	Stores          []RetailStoreSummary `json:"stores"`
	Monthly         []RetailMonthRow     `json:"monthly"`
	TotalSales15m   float64              `json:"totalSales15m"`
	SalesQ1_2023    float64              `json:"salesQ1_2023"`
	SalesQ1_2024    float64              `json:"salesQ1_2024"`
	YoYGrowthQ1Pct  float64              `json:"yoyGrowthQ1Pct"`
	TotalStock22_12 float64              `json:"totalStock22_12"`
	TotalStock23_06 float64              `json:"totalStock23_06"`
	TotalStock23_12 float64              `json:"totalStock23_12"`
	TotalStock24_03 float64              `json:"totalStock24_03"`
	TotalStock24_06 float64              `json:"totalStock24_06"`
	AvgStockMonths  float64              `json:"avgStockMonths"`
}

type RetailStoreSummary struct {
	City            string  `json:"city"`
	StoreName       string  `json:"storeName"`
	Stock22_12      float64 `json:"stock22_12"`
	Stock23_06      float64 `json:"stock23_06"`
	Stock23_12      float64 `json:"stock23_12"`
	Stock24_03      float64 `json:"stock24_03"`
	Stock24_06      float64 `json:"stock24_06"`
	Karcher24_03    float64 `json:"karcher24_03"`
	KarcherSharePct float64 `json:"karcherSharePct"`
	StockGrowthPct  float64 `json:"stockGrowthPct"` // 31.03.2024 vs 31.12.2022
	TotalSales15m   float64 `json:"totalSales15m"`
	SalesSharePct   float64 `json:"salesSharePct"`
	SalesQ1_2023    float64 `json:"salesQ1_2023"`
	SalesQ1_2024    float64 `json:"salesQ1_2024"`
	YoYGrowthQ1Pct  float64 `json:"yoyGrowthQ1Pct"`
	Avg6mSales      float64 `json:"avg6mSales"`
	StockMonths     float64 `json:"stockMonths"`
	TurnoverStatus  string  `json:"turnoverStatus"` // "optimal", "moderate", "high"
}

type RetailMonthRow struct {
	Period  string  `json:"period"`
	Vitebsk float64 `json:"vitebsk"`
	Grodno  float64 `json:"grodno"`
	Gomel   float64 `json:"gomel"`
	Brest   float64 `json:"brest"`
	Total   float64 `json:"total"`
}

// WarehousesData captures stock across central warehouses, retail, B2B regional branches, and service.
type WarehousesData struct {
	DateStart      string                `json:"dateStart"`
	DateEnd        string                `json:"dateEnd"`
	Items          []WarehouseRow        `json:"items"`
	ChannelSummary []WarehouseChannelAgg `json:"channelSummary"`
	TotalStart     float64               `json:"totalStart"`
	TotalEnd       float64               `json:"totalEnd"`
	TotalDelta     float64               `json:"totalDelta"`
	TotalDeltaPct  float64               `json:"totalDeltaPct"`
	RBServiceRows  []RBServiceRow        `json:"rbServiceRows"`
}

type WarehouseRow struct {
	Name       string  `json:"name"`
	Channel    string  `json:"channel"`
	StockStart float64 `json:"stockStart"`
	StockEnd   float64 `json:"stockEnd"`
	Delta      float64 `json:"delta"`
	DeltaPct   float64 `json:"deltaPct"`
	ShareEnd   float64 `json:"shareEnd"`
}

type WarehouseChannelAgg struct {
	Channel    string  `json:"channel"`
	StockStart float64 `json:"stockStart"`
	StockEnd   float64 `json:"stockEnd"`
	Delta      float64 `json:"delta"`
	DeltaPct   float64 `json:"deltaPct"`
	ShareEnd   float64 `json:"shareEnd"`
}

type RBServiceRow struct {
	Unit     string  `json:"unit"`
	Val2023  float64 `json:"val2023"`
	Val2024  float64 `json:"val2024"`
	Delta    float64 `json:"delta"`
	DeltaPct float64 `json:"deltaPct"`
}

// GroupsData captures the 20 supplier/brand stock groups + Russian supplier stock.
type GroupsData struct {
	Items            []GroupStockRow  `json:"items"`
	TrackedStart     float64          `json:"trackedStart"`
	TrackedEnd       float64          `json:"trackedEnd"`
	TrackedDelta     float64          `json:"trackedDelta"`
	TrackedGrowthPct float64          `json:"trackedGrowthPct"`
	CompanyTotalEnd  float64          `json:"companyTotalEnd"`
	CoverageEndPct   float64          `json:"coverageEndPct"`
	RFSupplierStocks []RFSupplierItem `json:"rfSupplierStocks"`
}

type GroupStockRow struct {
	Supplier  string  `json:"supplier"`
	Stock2023 float64 `json:"stock2023"`
	Stock2024 float64 `json:"stock2024"`
	Delta     float64 `json:"delta"`
	DeltaPct  float64 `json:"deltaPct"`
	SharePct  float64 `json:"sharePct"`
	CumShare  float64 `json:"cumShare"`
	ABCClass  string  `json:"abcClass"`
}

type RFSupplierItem struct {
	Supplier  string  `json:"supplier"`
	Stock2312 float64 `json:"stock2312"`
	Stock2403 float64 `json:"stock2403"`
	Delta     float64 `json:"delta"`
	DeltaPct  float64 `json:"deltaPct"`
}

// IntercompanyData captures SMG -> Chistiy Svet (RF) shipments & China product portfolio.
type IntercompanyData struct {
	Items      []IntercompanyRow `json:"items"`
	TotalQ4_23 float64           `json:"totalQ4_23"`
	TotalQ1_24 float64           `json:"totalQ1_24"`
	TotalDelta float64           `json:"totalDelta"`
	GrowthPct  float64           `json:"growthPct"`
	Top4Share  float64           `json:"top4Share"`
	ChinaItems []IntercompanyRow `json:"chinaItems"`
	ChinaQ4_23 float64           `json:"chinaQ4_23"`
	ChinaQ1_24 float64           `json:"chinaQ1_24"`
	ChinaDelta float64           `json:"chinaDelta"`
	ChinaPct   float64           `json:"chinaPct"`
}

type IntercompanyRow struct {
	Supplier string  `json:"supplier"`
	Q4_2023  float64 `json:"q4_2023"`
	Q1_2024  float64 `json:"q1_2024"`
	Delta    float64 `json:"delta"`
	DeltaPct float64 `json:"deltaPct"`
	ShareQ1  float64 `json:"shareQ1"`
	CumShare float64 `json:"cumShare"`
	ABCClass string  `json:"abcClass"`
}

// PrepaymentsData captures paid orders in transit, orders to pay, customs, and FX rates.
type PrepaymentsData struct {
	Rates              []FXRateItem     `json:"rates"`
	PaidOrders         []PrepaymentItem `json:"paidOrders"`
	UnpaidOrders       []PrepaymentItem `json:"unpaidOrders"`
	KarcherRFDebtRUB   float64          `json:"karcherRFDebtRUB"`
	KarcherRFDebtBYN   float64          `json:"karcherRFDebtBYN"`
	PaidPurchaseBYN    float64          `json:"paidPurchaseBYN"`
	PaidCustomsBYN     float64          `json:"paidCustomsBYN"`
	PaidTotalBYN       float64          `json:"paidTotalBYN"`
	UnpaidPurchaseBYN  float64          `json:"unpaidPurchaseBYN"`
	UnpaidCustomsBYN   float64          `json:"unpaidCustomsBYN"`
	UnpaidTotalBYN     float64          `json:"unpaidTotalBYN"`
	GrandTotalBYN      float64          `json:"grandTotalBYN"`
	CurrencyExposure   []CurrencyBucket `json:"currencyExposure"`
}

type FXRateItem struct {
	Code    string  `json:"code"` // USD, EUR, RUB, CNY, PLN
	Label   string  `json:"label"`
	UnitRate float64 `json:"unitRate"` // BYN per 1 unit of foreign currency
}

type PrepaymentItem struct {
	Status       string  `json:"status"` // "paid" or "unpaid"
	Supplier     string  `json:"supplier"`
	AmountOrig   float64 `json:"amountOrig"`
	Currency     string  `json:"currency"`
	CurrencyCode string  `json:"currencyCode"`
	CustomsOrig  float64 `json:"customsOrig"`
	CustomsRate  float64 `json:"customsRate"`
	PurchaseBYN  float64 `json:"purchaseBYN"`
	CustomsBYN   float64 `json:"customsBYN"`
	TotalBYN     float64 `json:"totalBYN"`
}

type CurrencyBucket struct {
	CurrencyCode string  `json:"currencyCode"`
	CurrencyName string  `json:"currencyName"`
	AmountOrig   float64 `json:"amountOrig"`
	TotalBYN     float64 `json:"totalBYN"`
	SharePct     float64 `json:"sharePct"`
}

// RFProcurementData captures Russian domestic procurement (Q4 2023 vs Q1 2024 + 1C drill-down).
type RFProcurementData struct {
	Period1C             string              `json:"period1C"`
	ExternalQ4_2023      float64             `json:"externalQ4_2023"`
	ExternalQ1_2024      float64             `json:"externalQ1_2024"`
	ExternalDelta        float64             `json:"externalDelta"`
	ExternalGrowthPct    float64             `json:"externalGrowthPct"`
	ComparisonRows       []RFComparisonRow   `json:"comparisonRows"`
	Suppliers1C          []RFSupplier1CRow   `json:"suppliers1C"`
	Monthly1C            []RFMonthly1CPoint  `json:"monthly1C"`
	TopSKUs              []RFSKURow          `json:"topSKUs"`
	LineItems            []RFLineItem        `json:"lineItems"`
	Total1CQty           float64             `json:"total1CQty"`
	Total1CNoVAT         float64             `json:"total1CNoVAT"`
	Total1CVAT           float64             `json:"total1CVAT"`
	Total1CWithVAT       float64             `json:"total1CWithVAT"`
	DocumentCount        int                 `json:"documentCount"`
	LineItemCount        int                 `json:"lineItemCount"`
}

type RFComparisonRow struct {
	Supplier string  `json:"supplier"`
	Category string  `json:"category"`
	Q4_2023  float64 `json:"q4_2023"`
	Q1_2024  float64 `json:"q1_2024"`
	Delta    float64 `json:"delta"`
	DeltaPct float64 `json:"deltaPct"`
	HasPct   bool    `json:"hasPct"`
}

type RFSupplier1CRow struct {
	Supplier   string  `json:"supplier"`
	Category   string  `json:"category"` // "Основные (+)", "Внутригрупповые / Керхер (-)", "Закупки сервис", "Разовые закупки"
	RawTag     string  `json:"rawTag"`
	DocCount   int     `json:"docCount"`
	ItemCount  int     `json:"itemCount"`
	Qty        float64 `json:"qty"`
	SumNoVAT   float64 `json:"sumNoVAT"`
	SumVAT     float64 `json:"sumVAT"`
	SumWithVAT float64 `json:"sumWithVAT"`
	SharePct   float64 `json:"sharePct"`
}

type RFMonthly1CPoint struct {
	Month        string  `json:"month"` // "Январь 2024", "Февраль 2024", "Март 2024"
	ExternalRUB  float64 `json:"externalRUB"`
	InternalRUB  float64 `json:"internalRUB"`
	TotalRUB     float64 `json:"totalRUB"`
	DocCount     int     `json:"docCount"`
}

type RFSKURow struct {
	SKU        string  `json:"sku"`
	Supplier   string  `json:"supplier"`
	Category   string  `json:"category"`
	Qty        float64 `json:"qty"`
	SumWithVAT float64 `json:"sumWithVAT"`
	OrderCount int     `json:"orderCount"`
}

type RFLineItem struct {
	Supplier   string  `json:"supplier"`
	Category   string  `json:"category"`
	DocNumber  string  `json:"docNumber"`
	DocDate    string  `json:"docDate"`
	Month      string  `json:"month"`
	SKU        string  `json:"sku"`
	VATRate    string  `json:"vatRate"`
	Qty        float64 `json:"qty"`
	SumNoVAT   float64 `json:"sumNoVAT"`
	SumVAT     float64 `json:"sumVAT"`
	SumWithVAT float64 `json:"sumWithVAT"`
}

// GenericSheetStat provides automatic column analytics for any arbitrary Excel table.
type GenericSheetStat struct {
	SheetName  string              `json:"sheetName"`
	RowCount   int                 `json:"rowCount"`
	ColCount   int                 `json:"colCount"`
	MetricCols []GenericMetricStat `json:"metricCols"`
	TopPreview []GenericLabelVal   `json:"topPreview"`
}

type GenericMetricStat struct {
	Header string  `json:"header"`
	Count  int     `json:"count"`
	Sum    float64 `json:"sum"`
	Avg    float64 `json:"avg"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
}

type GenericLabelVal struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

// RawSheetView contains a rendered grid for the universal Excel sheet browser.
type RawSheetView struct {
	Name      string          `json:"name"`
	MaxRow    int             `json:"maxRow"`
	MaxCol    int             `json:"maxCol"`
	Truncated bool            `json:"truncated"`
	Rows      [][]RawCellView `json:"rows"`
}

type RawCellView struct {
	Text    string `json:"t,omitempty"`
	Formula string `json:"f,omitempty"`
	IsNum   bool   `json:"n,omitempty"`
	IsPct   bool   `json:"p,omitempty"`
	Indent  int    `json:"i,omitempty"`
}
