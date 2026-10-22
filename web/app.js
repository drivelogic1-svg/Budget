(function () {
  const state = {
    report: null,
    desktopMode: false,
    hasNativeDialog: false,
    activeTab: "overview",
    overviewPeriod: "all",
    karcherPeriod: "all",
    retailCity: "all",
    warehouseChannel: "all",
    groupsABC: "all",
    groupsQuery: "",
    rfCategory: "all",
    rfMonth: "all",
    rfQuery: "",
    rfPage: 1,
    fxRates: {},
    baseFxRates: {},
    selectedSheetIdx: 0,
    sheetQuery: ""
  };

  const TAB_META = {
    overview: {
      title: "Сводная аналитика продаж и товарных запасов",
      subtitle: "Первое полугодие 2024 г. · Реализация по себестоимости (BYN без НДС), динамика склада и оборачиваемость",
      csvSection: "summary"
    },
    karcher: {
      title: "Направление Керхер: продажи, закупки и сегменты H&G / Проф",
      subtitle: "Помесячный баланс реализации и закупок за 18 месяцев (Январь 2023 – Июнь 2024), накопление запасов",
      csvSection: "karcher"
    },
    retail: {
      title: "Розничная сеть: Брест, Гомель, Гродно, Витебск",
      subtitle: "Динамика розничных продаж за 15 месяцев, рост YoY и оборачиваемость товарного запаса магазинов",
      csvSection: "retail"
    },
    intercompany: {
      title: "Отгрузки СМГ → ЧСТ (РФ) и портфель Поставщики Китай",
      subtitle: "Межфирменные продажи на РФ (4 кв. 2023 vs 1 кв. 2024), ABC-концентрация собственного производства",
      csvSection: "intercompany"
    },
    warehouses: {
      title: "Склады, отделы продаж B2B и сервисно-производственный блок",
      subtitle: "Распределение запасов по каналам сбыта (31.12.2023 → 30.06.2024) и выработка подразделений РБ",
      csvSection: "warehouses"
    },
    groups: {
      title: "Товарные группы и ABC-анализ вложенного капитала",
      subtitle: "20 брендов и групп поставщиков (94% всего склада компании) + детализация российских поставщиков",
      csvSection: "groups"
    },
    rf: {
      title: "Закупки РФ-РФ и детализация 1С до документа и SKU",
      subtitle: "Сравнение 4 кв. 2023 vs 1 кв. 2024 и полный реестр поступлений 1С (RUB с НДС)",
      csvSection: "rf1c"
    },
    prepayments: {
      title: "Предварительные оплаты, таможня и валютный калькулятор",
      subtitle: "Оплаченные заказы в пути, заявки к оплате, долг Керхер РФ и стресс-тест валютных курсов",
      csvSection: "prepayments"
    },
    sheets: {
      title: "Просмотр всех исходных листов Excel",
      subtitle: "Полное содержимое вкладок документа со значениями, отступами и формулами Excel",
      csvSection: "sheet"
    }
  };

  const fmtMoney = (v, suffix) => window.SalesCharts.fullNum(v, suffix);
  const fmtCompact = (v, suffix) => window.SalesCharts.compactNum(v, suffix);

  function fmtPct(v, signed = true) {
    if (v == null || isNaN(v)) return "—";
    const sign = signed && v > 0 ? "+" : "";
    return sign + v.toFixed(1).replace(".", ",") + "%";
  }

  function deltaBadge(pct, invertWarning) {
    if (pct == null || isNaN(pct)) return `<span class="badge badge-neutral">—</span>`;
    let cls = pct > 0 ? "badge-pos" : pct < 0 ? "badge-neg" : "badge-neutral";
    if (invertWarning && pct > 20) cls = "badge-warn";
    return `<span class="badge ${cls}">${fmtPct(pct, true)}</span>`;
  }

  function esc(s) {
    return String(s == null ? "" : s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function showToast(msg) {
    const el = document.getElementById("toast");
    el.textContent = msg;
    el.classList.add("show");
    clearTimeout(showToast._t);
    showToast._t = setTimeout(() => el.classList.remove("show"), 3400);
  }

  function initTheme() {
    const saved = localStorage.getItem("ba_theme") || "light";
    document.documentElement.setAttribute("data-theme", saved);
  }

  function toggleTheme() {
    const cur = document.documentElement.getAttribute("data-theme") === "dark" ? "light" : "dark";
    document.documentElement.setAttribute("data-theme", cur);
    localStorage.setItem("ba_theme", cur);
    renderCurrentTab();
  }

  async function loadDashboard() {
    try {
      const res = await fetch("api/dashboard");
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "Ошибка загрузки данных");
      applyPayload(data);
    } catch (err) {
      document.getElementById("app-view").innerHTML = `
        <div class="panel"><div class="panel-body" style="color:var(--danger)">
          Ошибка загрузки аналитических данных: ${esc(err.message)}
        </div></div>`;
    }
  }

  function applyPayload(data) {
    state.report = data.report;
    state.desktopMode = !!data.desktopMode;
    state.hasNativeDialog = !!data.hasNativeDialog;
    state.selectedSheetIdx = 0;
    state.rfPage = 1;

    // Initialize FX simulator rates from workbook
    state.baseFxRates = { USD: 3.2047, EUR: 3.4725, RUB: 0.036462, CNY: 0.43975, PLN: 0.81388 };
    if (state.report && state.report.prepayments && state.report.prepayments.rates) {
      state.report.prepayments.rates.forEach((r) => {
        if (r.code && r.unitRate > 0) state.baseFxRates[r.code] = r.unitRate;
      });
    }
    state.fxRates = Object.assign({}, state.baseFxRates);

    updateChrome();
    renderCurrentTab();
  }

  function updateChrome() {
    const rep = state.report;
    if (!rep) return;
    document.getElementById("sidebar-filename").textContent = rep.fileName;
    document.getElementById("sidebar-filemeta").textContent = `${rep.sheetCount} листов · ${rep.loadedAt}`;
    document.getElementById("badge-sheets-count").textContent = String(rep.sheetCount);
    const rfCnt = rep.rfProcurement && rep.rfProcurement.lineItemCount ? rep.rfProcurement.lineItemCount : 0;
    document.getElementById("badge-rf-count").textContent = rfCnt > 0 ? String(rfCnt) : "1С";
  }

  function switchTab(tab) {
    state.activeTab = tab;
    document.querySelectorAll(".nav-btn").forEach((btn) => {
      btn.classList.toggle("active", btn.dataset.tab === tab);
    });
    const meta = TAB_META[tab] || TAB_META.overview;
    document.getElementById("page-title").textContent = meta.title;
    document.getElementById("page-subtitle").textContent = meta.subtitle;
    renderCurrentTab();
  }

  function renderCurrentTab() {
    const rep = state.report;
    if (!rep) return;
    const container = document.getElementById("app-view");

    if (!rep.isBudgetDataset && state.activeTab !== "sheets") {
      renderGenericWorkbook(container, rep);
      return;
    }

    switch (state.activeTab) {
      case "overview":
        renderOverview(container, rep);
        break;
      case "karcher":
        renderKarcher(container, rep);
        break;
      case "retail":
        renderRetail(container, rep);
        break;
      case "intercompany":
        renderIntercompany(container, rep);
        break;
      case "warehouses":
        renderWarehouses(container, rep);
        break;
      case "groups":
        renderGroups(container, rep);
        break;
      case "rf":
        renderRF(container, rep);
        break;
      case "prepayments":
        renderPrepayments(container, rep);
        break;
      case "sheets":
        renderSheets(container, rep);
        break;
      default:
        renderOverview(container, rep);
    }
  }

  // ==================== 1. OVERVIEW ====================
  function renderOverview(el, rep) {
    const k = rep.kpis;
    const si = rep.salesInventory;
    const months = si.monthlySales.filter((m) => state.overviewPeriod === "all" || m.period === state.overviewPeriod);

    el.innerHTML = `
      <div class="kpi-grid">
        <div class="kpi-card kpi-positive">
          <div class="kpi-label">Реализация за 1П 2024 (себест.)</div>
          <div class="kpi-value">${fmtCompact(k.salesH1_2024, "BYN")}</div>
          <div class="kpi-meta">
            <span>2П 2023: ${fmtCompact(k.salesH2_2023, "BYN")}</span>
            ${deltaBadge(k.salesGrowthPct)}
          </div>
        </div>
        <div class="kpi-card kpi-info">
          <div class="kpi-label">Среднемесячные продажи 1П 2024</div>
          <div class="kpi-value">${fmtCompact(k.avgMonthlySalesH1_24, "BYN")}</div>
          <div class="kpi-meta">
            <span>Пик: ${esc(k.peakSalesMonth)} (${fmtCompact(k.peakSalesValue, "")})</span>
          </div>
        </div>
        <div class="kpi-card kpi-warning">
          <div class="kpi-label">Остатки на складе (30.06.2024)</div>
          <div class="kpi-value">${fmtCompact(k.stockMid2024, "BYN")}</div>
          <div class="kpi-meta">
            <span>31.12.2023: ${fmtCompact(k.stockEnd2023, "BYN")}</span>
            ${deltaBadge(k.stockGrowthH1_2024Pct, true)}
          </div>
        </div>
        <div class="kpi-card ${k.stockMonthsMid2024 > 3.3 ? "kpi-warning" : "kpi-positive"}">
          <div class="kpi-label">Оборачиваемость (запас в мес.)</div>
          <div class="kpi-value">${k.stockMonthsMid2024.toFixed(2).replace(".", ",")} мес.</div>
          <div class="kpi-meta">
            <span>Коэф. ${k.annualTurnoverRatio.toFixed(2).replace(".", ",")}x/год · было ${k.stockMonthsEnd2023.toFixed(2).replace(".", ",")} мес.</span>
          </div>
        </div>
        <div class="kpi-card kpi-positive">
          <div class="kpi-label">Керхер в реализации 1П 2024</div>
          <div class="kpi-value">${fmtCompact(k.karcherSalesH1_2024, "BYN")}</div>
          <div class="kpi-meta">
            <span>Доля ${k.karcherShareH1_2024.toFixed(1).replace(".", ",")}% компании</span>
            <span class="badge badge-pos">YoY ${fmtPct(k.karcherYoYGrowthPct)}</span>
          </div>
        </div>
        <div class="kpi-card kpi-danger">
          <div class="kpi-label">Предоплаты и обязательства</div>
          <div class="kpi-value">${fmtCompact(k.totalCommitmentsBYN, "BYN")}</div>
          <div class="kpi-meta">
            <span>В пути + к оплате + Керхер РФ</span>
            <span class="badge badge-info">СМГ→ЧСТ ${fmtPct(k.smgToChstGrowthPct)}</span>
          </div>
        </div>
      </div>

      <div class="panel">
        <div class="panel-header">
          <div>
            <div class="panel-title">Ключевые аналитические выводы по документу</div>
            <div class="panel-subtitle">Автоматическая диагностика продаж, товарных запасов, каналов сбыта и финансовых обязательств</div>
          </div>
        </div>
        <div class="panel-body">
          <div class="insights-grid">
            ${rep.insights.map((ins) => {
              const bCls = ins.severity === "positive" ? "badge-pos" : ins.severity === "warning" ? "badge-warn" : ins.severity === "critical" ? "badge-neg" : "badge-info";
              return `
                <div class="insight-card">
                  <div class="insight-top">
                    <span class="insight-cat">${esc(ins.category)}</span>
                    <span class="badge ${bCls}">${esc(ins.metric)}</span>
                  </div>
                  <div class="insight-title">${esc(ins.title)}</div>
                  <div class="insight-detail">${esc(ins.detail)}</div>
                </div>`;
            }).join("")}
          </div>
        </div>
      </div>

      <div class="grid-65-35">
        <div class="panel">
          <div class="panel-header">
            <div>
              <div class="panel-title">Помесячная динамика реализации компании (BYN без НДС)</div>
              <div class="panel-subtitle">Структура продаж: направление Керхер и остальной ассортимент + средние уровни полугодий</div>
            </div>
            <div class="segmented" id="seg-overview-period">
              <button class="seg-btn ${state.overviewPeriod === "all" ? "active" : ""}" data-val="all">Все 12 мес.</button>
              <button class="seg-btn ${state.overviewPeriod === "2П 2023" ? "active" : ""}" data-val="2П 2023">2П 2023</button>
              <button class="seg-btn ${state.overviewPeriod === "1П 2024" ? "active" : ""}" data-val="1П 2024">1П 2024</button>
            </div>
          </div>
          <div class="panel-body">
            <div class="chart-wrap"><canvas id="chart-overview-sales" class="chart-canvas"></canvas></div>
          </div>
        </div>

        <div class="panel">
          <div class="panel-header">
            <div>
              <div class="panel-title">Рост складских остатков (2022–2024)</div>
              <div class="panel-subtitle">По себестоимости, бел. руб. без НДС (+46,5% за 18 мес.)</div>
            </div>
          </div>
          <div class="panel-body">
            <div class="chart-wrap"><canvas id="chart-overview-stock" class="chart-canvas"></canvas></div>
          </div>
        </div>
      </div>

      <div class="panel">
        <div class="panel-header">
          <div>
            <div class="panel-title">Детализация помесячной реализации (Июль 2023 – Июнь 2024)</div>
            <div class="panel-subtitle">Сопоставление общей реализации из листа «Остатки всего» и продаж направления «Керхер»</div>
          </div>
        </div>
        <div class="table-scroll">
          <table class="data-table">
            <thead>
              <tr>
                <th>Полугодие</th>
                <th>Месяц</th>
                <th class="num">Реализация всего, BYN</th>
                <th class="num">В т.ч. Керхер, BYN</th>
                <th class="num">Прочий ассортимент, BYN</th>
                <th class="num">Доля Керхер</th>
                <th class="num">Динамика к пред. мес.</th>
              </tr>
            </thead>
            <tbody>
              ${months.map((m) => `
                <tr>
                  <td><span class="badge ${m.period === "1П 2024" ? "badge-info" : "badge-neutral"}">${esc(m.period)}</span></td>
                  <td><strong>${esc(m.fullLabel)}</strong></td>
                  <td class="num"><strong>${fmtMoney(m.totalSales)}</strong></td>
                  <td class="num">${fmtMoney(m.karcherSales)}</td>
                  <td class="num">${fmtMoney(m.otherSales)}</td>
                  <td class="num">${m.karcherShare.toFixed(1).replace(".", ",")}%</td>
                  <td class="num">${m.momGrowthPct === 0 ? "—" : deltaBadge(m.momGrowthPct)}</td>
                </tr>`).join("")}
            </tbody>
            <tfoot>
              <tr>
                <td colspan="2">ИТОГО за выбранный период (${months.length} мес.)</td>
                <td class="num">${fmtMoney(months.reduce((a, b) => a + b.totalSales, 0))}</td>
                <td class="num">${fmtMoney(months.reduce((a, b) => a + b.karcherSales, 0))}</td>
                <td class="num">${fmtMoney(months.reduce((a, b) => a + b.otherSales, 0))}</td>
                <td class="num">${(months.reduce((a, b) => a + b.karcherSales, 0) / Math.max(1, months.reduce((a, b) => a + b.totalSales, 0)) * 100).toFixed(1).replace(".", ",")}%</td>
                <td class="num">${deltaBadge(k.salesGrowthPct)}</td>
              </tr>
            </tfoot>
          </table>
        </div>
      </div>
    `;

    el.querySelectorAll("#seg-overview-period .seg-btn").forEach((btn) => {
      btn.onclick = () => {
        state.overviewPeriod = btn.dataset.val;
        renderOverview(el, rep);
      };
    });

    window.SalesCharts.renderComboChart(document.getElementById("chart-overview-sales"), {
      labels: months.map((m) => m.fullLabel),
      stacked: true,
      unit: "BYN",
      series: [
        { name: "Керхер", type: "bar", color: "#2563eb", data: months.map((m) => m.karcherSales) },
        { name: "Прочий ассортимент", type: "bar", color: "#38bdf8", data: months.map((m) => m.otherSales) },
        { name: "Всего реализация", type: "line", color: "#0f172a", data: months.map((m) => m.totalSales) }
      ],
      refLines: [
        { value: k.avgMonthlySalesH1_24, label: "Средняя 1П 2024: " + fmtCompact(k.avgMonthlySalesH1_24, ""), color: "#10b981" },
        { value: k.avgMonthlySalesH2_23, label: "Средняя 2П 2023: " + fmtCompact(k.avgMonthlySalesH2_23, ""), color: "#f59e0b" }
      ]
    });

    window.SalesCharts.renderComboChart(document.getElementById("chart-overview-stock"), {
      labels: si.stockCheckpoints.map((c) => c.dateLabel),
      unit: "BYN",
      series: [
        { name: "Остатки на складе", type: "bar", color: "#f59e0b", data: si.stockCheckpoints.map((c) => c.valueBYN) }
      ]
    });
  }

  // ==================== 2. KARCHER ====================
  function renderKarcher(el, rep) {
    const kd = rep.karcher;
    const rows = kd.monthly.filter((m) => state.karcherPeriod === "all" || m.halfYear === state.karcherPeriod);
    const yoy = kd.salesH1_2023 > 0 ? ((kd.salesH1_2024 / kd.salesH1_2023 - 1) * 100) : 0;

    el.innerHTML = `
      <div class="kpi-grid">
        <div class="kpi-card kpi-positive">
          <div class="kpi-label">Реализация Керхер за 18 мес.</div>
          <div class="kpi-value">${fmtCompact(kd.totalSales18m, "BYN")}</div>
          <div class="kpi-meta"><span>1П 2024: ${fmtCompact(kd.salesH1_2024, "BYN")}</span><span class="badge badge-pos">YoY ${fmtPct(yoy)}</span></div>
        </div>
        <div class="kpi-card kpi-info">
          <div class="kpi-label">Сумма закупок за 18 мес.</div>
          <div class="kpi-value">${fmtCompact(kd.totalPurch18m, "BYN")}</div>
          <div class="kpi-meta"><span>1П 2024: ${fmtCompact(kd.purchH1_2024, "BYN")}</span></div>
        </div>
        <div class="kpi-card kpi-warning">
          <div class="kpi-label">Превышение закупок над продажами</div>
          <div class="kpi-value">+${fmtCompact(kd.totalDiff18m, "BYN")}</div>
          <div class="kpi-meta"><span>За 1П 2024: +${fmtCompact(kd.netBuildH1_2024, "BYN")}</span></div>
        </div>
        <div class="kpi-card kpi-info">
          <div class="kpi-label">Остатки КЕРХЕР H&G / Проф (30.06.24)</div>
          <div class="kpi-value">${fmtCompact(kd.stockHG_Mid2024 + kd.stockProfMid2024, "BYN")}</div>
          <div class="kpi-meta"><span>H&G: ${fmtCompact(kd.stockHG_Mid2024, "")} · Проф: ${fmtCompact(kd.stockProfMid2024, "")}</span></div>
        </div>
        ${kd.soldToKarcherRF > 0 ? `
        <div class="kpi-card kpi-positive">
          <div class="kpi-label">Доп. отгрузка Керхер РФ</div>
          <div class="kpi-value">${fmtCompact(kd.soldToKarcherRF, "BYN")}</div>
          <div class="kpi-meta"><span>${esc(kd.soldRFNote || "с 21.06")}</span></div>
        </div>` : ""}
      </div>

      <div class="grid-2">
        <div class="panel">
          <div class="panel-header">
            <div>
              <div class="panel-title">Реализация vs Закупки Керхер (BYN без НДС)</div>
              <div class="panel-subtitle">Сопоставление ежемесячного объёма продаж и пополнения склада</div>
            </div>
            <div class="segmented" id="seg-karcher-period">
              <button class="seg-btn ${state.karcherPeriod === "all" ? "active" : ""}" data-val="all">Все 18 мес.</button>
              <button class="seg-btn ${state.karcherPeriod === "1П 2023" ? "active" : ""}" data-val="1П 2023">1П 2023</button>
              <button class="seg-btn ${state.karcherPeriod === "2П 2023" ? "active" : ""}" data-val="2П 2023">2П 2023</button>
              <button class="seg-btn ${state.karcherPeriod === "1П 2024" ? "active" : ""}" data-val="1П 2024">1П 2024</button>
            </div>
          </div>
          <div class="panel-body">
            <div class="chart-wrap"><canvas id="chart-karcher-main" class="chart-canvas"></canvas></div>
          </div>
        </div>

        <div class="panel">
          <div class="panel-header">
            <div>
              <div class="panel-title">Сегменты КЕРХЕР H&G (бытовой) и КЕРХЕР Проф (2023 г.)</div>
              <div class="panel-subtitle">Структура помесячной реализации по двум бизнес-направлениям бренда</div>
            </div>
          </div>
          <div class="panel-body">
            <div class="chart-wrap"><canvas id="chart-karcher-seg" class="chart-canvas"></canvas></div>
          </div>
        </div>
      </div>

      <div class="panel">
        <div class="panel-header">
          <div>
            <div class="panel-title">Помесячная таблица баланса продаж и закупок Керхер</div>
            <div class="panel-subtitle">Положительная разница означает рост складского запаса, отрицательная — распродажу запаса</div>
          </div>
        </div>
        <div class="table-scroll">
          <table class="data-table">
            <thead>
              <tr>
                <th>Месяц</th>
                <th>Полугодие</th>
                <th class="num">Реализация, BYN</th>
                <th class="num">Сумма закупки, BYN</th>
                <th class="num">Закупки − Реализация, BYN</th>
                <th class="num">Накопленное сальдо, BYN</th>
                <th class="num">Закупки / Продажи</th>
              </tr>
            </thead>
            <tbody>
              ${rows.map((r) => `
                <tr>
                  <td><strong>${esc(r.month)}</strong></td>
                  <td><span class="badge badge-neutral">${esc(r.halfYear)}</span></td>
                  <td class="num">${fmtMoney(r.salesBYN)}</td>
                  <td class="num">${fmtMoney(r.purchasesBYN)}</td>
                  <td class="num" style="color:${r.diffBYN > 0 ? "var(--warning)" : "var(--positive)"};font-weight:600">${r.diffBYN > 0 ? "+" : ""}${fmtMoney(r.diffBYN)}</td>
                  <td class="num">${fmtMoney(r.cumDiffBYN)}</td>
                  <td class="num"><span class="badge ${r.purchToSales > 115 ? "badge-warn" : "badge-pos"}">${r.purchToSales.toFixed(1).replace(".", ",")}%</span></td>
                </tr>`).join("")}
            </tbody>
            <tfoot>
              <tr>
                <td colspan="2">ИТОГО (${rows.length} мес.)</td>
                <td class="num">${fmtMoney(rows.reduce((a, b) => a + b.salesBYN, 0))}</td>
                <td class="num">${fmtMoney(rows.reduce((a, b) => a + b.purchasesBYN, 0))}</td>
                <td class="num">${fmtMoney(rows.reduce((a, b) => a + b.diffBYN, 0))}</td>
                <td class="num">—</td>
                <td class="num">${(rows.reduce((a, b) => a + b.purchasesBYN, 0) / Math.max(1, rows.reduce((a, b) => a + b.salesBYN, 0)) * 100).toFixed(1).replace(".", ",")}%</td>
              </tr>
            </tfoot>
          </table>
        </div>
      </div>
    `;

    el.querySelectorAll("#seg-karcher-period .seg-btn").forEach((btn) => {
      btn.onclick = () => {
        state.karcherPeriod = btn.dataset.val;
        renderKarcher(el, rep);
      };
    });

    window.SalesCharts.renderComboChart(document.getElementById("chart-karcher-main"), {
      labels: rows.map((r) => r.month),
      unit: "BYN",
      series: [
        { name: "Реализация", type: "bar", color: "#10b981", data: rows.map((r) => r.salesBYN) },
        { name: "Закупки", type: "bar", color: "#2563eb", data: rows.map((r) => r.purchasesBYN) },
        { name: "Накопл. сальдо склада", type: "line", color: "#f59e0b", data: rows.map((r) => r.cumDiffBYN) }
      ]
    });

    window.SalesCharts.renderComboChart(document.getElementById("chart-karcher-seg"), {
      labels: kd.segments2023.map((s) => s.month),
      stacked: true,
      unit: "BYN",
      series: [
        { name: "КЕРХЕР H&G", type: "bar", color: "#f59e0b", data: kd.segments2023.map((s) => s.hg) },
        { name: "КЕРХЕР Проф", type: "bar", color: "#2563eb", data: kd.segments2023.map((s) => s.prof) }
      ]
    });
  }

  // ==================== 3. RETAIL ====================
  function renderRetail(el, rep) {
    const rd = rep.retail;
    const statusLabel = (st) =>
      st === "optimal" ? `<span class="badge badge-pos">Оптимально</span>`
      : st === "moderate" ? `<span class="badge badge-info">Умеренно</span>`
      : `<span class="badge badge-warn">Избыток запаса</span>`;

    el.innerHTML = `
      <div class="kpi-grid">
        <div class="kpi-card kpi-positive">
          <div class="kpi-label">Продажи розницы (15 мес.)</div>
          <div class="kpi-value">${fmtCompact(rd.totalSales15m, "BYN")}</div>
          <div class="kpi-meta"><span>Январь 2023 – Март 2024 (4 магазина)</span></div>
        </div>
        <div class="kpi-card kpi-positive">
          <div class="kpi-label">Продажи розницы 1 кв. 2024</div>
          <div class="kpi-value">${fmtCompact(rd.salesQ1_2024, "BYN")}</div>
          <div class="kpi-meta"><span>1 кв. 2023: ${fmtCompact(rd.salesQ1_2023, "BYN")}</span>${deltaBadge(rd.yoyGrowthQ1Pct)}</div>
        </div>
        <div class="kpi-card kpi-info">
          <div class="kpi-label">Остатки розницы (31.03 / 30.06.24)</div>
          <div class="kpi-value">${fmtCompact(rd.totalStock24_03, "BYN")}</div>
          <div class="kpi-meta"><span>На 30.06.2024: ${fmtCompact(rd.totalStock24_06, "BYN")}</span></div>
        </div>
        <div class="kpi-card kpi-warning">
          <div class="kpi-label">Средний запас розницы</div>
          <div class="kpi-value">${rd.avgStockMonths.toFixed(2).replace(".", ",")} мес.</div>
          <div class="kpi-meta"><span>От 2,16 мес. (Брест) до 3,55 мес. (Витебск)</span></div>
        </div>
      </div>

      <div class="grid-2">
        <div class="panel">
          <div class="panel-header">
            <div>
              <div class="panel-title">Помесячная реализация розничных магазинов (BYN без НДС)</div>
              <div class="panel-subtitle">Сезонность спроса (весенний пик в марте–апреле) и вклад городов</div>
            </div>
            <div class="segmented" id="seg-retail-city">
              <button class="seg-btn ${state.retailCity === "all" ? "active" : ""}" data-val="all">Все города</button>
              <button class="seg-btn ${state.retailCity === "Брест" ? "active" : ""}" data-val="Брест">Брест</button>
              <button class="seg-btn ${state.retailCity === "Гомель" ? "active" : ""}" data-val="Гомель">Гомель</button>
              <button class="seg-btn ${state.retailCity === "Гродно" ? "active" : ""}" data-val="Гродно">Гродно</button>
              <button class="seg-btn ${state.retailCity === "Витебск" ? "active" : ""}" data-val="Витебск">Витебск</button>
            </div>
          </div>
          <div class="panel-body">
            <div class="chart-wrap"><canvas id="chart-retail-monthly" class="chart-canvas"></canvas></div>
          </div>
        </div>

        <div class="panel">
          <div class="panel-header">
            <div>
              <div class="panel-title">Сравнение продаж 1 кв. 2023 vs 1 кв. 2024 по городам</div>
              <div class="panel-subtitle">Рост год-к-году (YoY) в первом квартале по каждому магазину</div>
            </div>
          </div>
          <div class="panel-body">
            <div class="chart-wrap"><canvas id="chart-retail-yoy" class="chart-canvas"></canvas></div>
          </div>
        </div>
      </div>

      <div class="panel">
        <div class="panel-header">
          <div>
            <div class="panel-title">Рейтинг эффективности розничных магазинов</div>
            <div class="panel-subtitle">Продажи, доля Керхер в стоке и покрытие среднемесячной реализации товарным остатком</div>
          </div>
        </div>
        <div class="table-scroll">
          <table class="data-table">
            <thead>
              <tr>
                <th>Магазин</th>
                <th class="num">Продажи 15 мес., BYN</th>
                <th class="num">Доля сети</th>
                <th class="num">1 кв. 2023</th>
                <th class="num">1 кв. 2024</th>
                <th class="num">Рост YoY</th>
                <th class="num">Средн. мес. (6м)</th>
                <th class="num">Остаток 31.03.24</th>
                <th class="num">Остаток 30.06.24</th>
                <th class="num">Доля Керхер</th>
                <th class="num">Запас (мес.)</th>
                <th>Статус</th>
              </tr>
            </thead>
            <tbody>
              ${rd.stores.map((st) => `
                <tr>
                  <td><strong>${esc(st.storeName)}</strong></td>
                  <td class="num"><strong>${fmtMoney(st.totalSales15m)}</strong></td>
                  <td class="num">${st.salesSharePct.toFixed(1).replace(".", ",")}%</td>
                  <td class="num">${fmtMoney(st.salesQ1_2023)}</td>
                  <td class="num">${fmtMoney(st.salesQ1_2024)}</td>
                  <td class="num">${deltaBadge(st.yoyGrowthQ1Pct)}</td>
                  <td class="num">${fmtMoney(st.avg6mSales)}</td>
                  <td class="num">${fmtMoney(st.stock24_03)}</td>
                  <td class="num">${fmtMoney(st.stock24_06)}</td>
                  <td class="num">${st.karcherSharePct.toFixed(1).replace(".", ",")}%</td>
                  <td class="num"><strong>${st.stockMonths.toFixed(2).replace(".", ",")}</strong></td>
                  <td>${statusLabel(st.turnoverStatus)}</td>
                </tr>`).join("")}
            </tbody>
          </table>
        </div>
      </div>
    `;

    el.querySelectorAll("#seg-retail-city .seg-btn").forEach((btn) => {
      btn.onclick = () => {
        state.retailCity = btn.dataset.val;
        renderRetail(el, rep);
      };
    });

    const citySeriesMap = {
      "Брест": { name: "Брест", type: "bar", color: "#2563eb", data: rd.monthly.map((m) => m.brest) },
      "Гомель": { name: "Гомель", type: "bar", color: "#10b981", data: rd.monthly.map((m) => m.gomel) },
      "Гродно": { name: "Гродно", type: "bar", color: "#f59e0b", data: rd.monthly.map((m) => m.grodno) },
      "Витебск": { name: "Витебск", type: "bar", color: "#8b5cf6", data: rd.monthly.map((m) => m.vitebsk) }
    };
    const mSeries = state.retailCity === "all"
      ? [
          citySeriesMap["Брест"],
          citySeriesMap["Гомель"],
          citySeriesMap["Гродно"],
          citySeriesMap["Витебск"],
          { name: "Итого сеть", type: "line", color: "#0f172a", data: rd.monthly.map((m) => m.total) }
        ]
      : [citySeriesMap[state.retailCity]];

    window.SalesCharts.renderComboChart(document.getElementById("chart-retail-monthly"), {
      labels: rd.monthly.map((m) => m.period),
      stacked: state.retailCity === "all",
      unit: "BYN",
      series: mSeries
    });

    window.SalesCharts.renderComboChart(document.getElementById("chart-retail-yoy"), {
      labels: rd.stores.map((s) => s.city),
      unit: "BYN",
      series: [
        { name: "1 кв. 2023", type: "bar", color: "#94a3b8", data: rd.stores.map((s) => s.salesQ1_2023) },
        { name: "1 кв. 2024", type: "bar", color: "#2563eb", data: rd.stores.map((s) => s.salesQ1_2024) }
      ]
    });
  }

  // ==================== 4. INTERCOMPANY (SMG -> ChST & CHINA) ====================
  function renderIntercompany(el, rep) {
    const ic = rep.intercompany;

    el.innerHTML = `
      <div class="kpi-grid">
        <div class="kpi-card kpi-positive">
          <div class="kpi-label">Отгрузки СМГ → ЧСТ (1 кв. 2024)</div>
          <div class="kpi-value">${fmtCompact(ic.totalQ1_24, "BYN")}</div>
          <div class="kpi-meta"><span>4 кв. 2023: ${fmtCompact(ic.totalQ4_23, "BYN")}</span>${deltaBadge(ic.growthPct)}</div>
        </div>
        <div class="kpi-card kpi-info">
          <div class="kpi-label">Прирост поставок за квартал</div>
          <div class="kpi-value">+${fmtCompact(ic.totalDelta, "BYN")}</div>
          <div class="kpi-meta"><span>По себестоимости без НДС</span></div>
        </div>
        <div class="kpi-card kpi-positive">
          <div class="kpi-label">Концентрация Топ-4 групп (Парето)</div>
          <div class="kpi-value">${ic.top4Share.toFixed(1).replace(".", ",")}%</div>
          <div class="kpi-meta"><span>Собственные системы + ЗЧ = 66,4%</span></div>
        </div>
        <div class="kpi-card kpi-info">
          <div class="kpi-label">Поставщики Китай (1 кв. 2024)</div>
          <div class="kpi-value">${fmtCompact(ic.chinaQ1_24, "BYN")}</div>
          <div class="kpi-meta"><span>4 кв. 2023: ${fmtCompact(ic.chinaQ4_23, "BYN")}</span>${deltaBadge(ic.chinaPct)}</div>
        </div>
      </div>

      <div class="grid-65-35">
        <div class="panel">
          <div class="panel-header">
            <div>
              <div class="panel-title">Кривая Парето (ABC) и динамика отгрузок СМГ → ЧСТ</div>
              <div class="panel-subtitle">Сравнение 4 кв. 2023 и 1 кв. 2024 (BYN) + накопленная доля выручки (%)</div>
            </div>
          </div>
          <div class="panel-body">
            <div class="chart-wrap chart-wrap-tall"><canvas id="chart-ic-pareto" class="chart-canvas"></canvas></div>
          </div>
        </div>

        <div class="panel">
          <div class="panel-header">
            <div>
              <div class="panel-title">Портфель «Поставщики Китай»</div>
              <div class="panel-subtitle">Динамика 1 кв. 2024 к 4 кв. 2023 (BYN без НДС)</div>
            </div>
          </div>
          <div class="panel-body">
            <div class="chart-wrap chart-wrap-sm"><canvas id="chart-ic-china" class="chart-canvas"></canvas></div>
            <div class="table-scroll" style="margin-top:10px">
              <table class="data-table">
                <thead>
                  <tr><th>Номенклатурная группа</th><th class="num">4 кв. 23</th><th class="num">1 кв. 24</th><th class="num">Рост</th></tr>
                </thead>
                <tbody>
                  ${ic.chinaItems.map((c) => `
                    <tr>
                      <td>${esc(c.supplier)}</td>
                      <td class="num">${fmtMoney(c.q4_2023)}</td>
                      <td class="num"><strong>${fmtMoney(c.q1_2024)}</strong></td>
                      <td class="num">${c.q4_2023 > 0 ? deltaBadge(c.deltaPct) : "—"}</td>
                    </tr>`).join("")}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>

      <div class="panel">
        <div class="panel-header">
          <div>
            <div class="panel-title">Сводная таблица межфирменных продаж СМГ → ЧСТ</div>
            <div class="panel-subtitle">Ранжирование по объёму 1-го квартала 2024 года с ABC-классификацией</div>
          </div>
        </div>
        <div class="table-scroll">
          <table class="data-table">
            <thead>
              <tr>
                <th>ABC</th>
                <th>Группа / Поставщик</th>
                <th class="num">4-й квартал 2023, BYN</th>
                <th class="num">1-й квартал 2024, BYN</th>
                <th class="num">Динамика, BYN</th>
                <th class="num">Динамика, %</th>
                <th class="num">Доля 1 кв. 2024</th>
                <th class="num">Накопленная доля</th>
              </tr>
            </thead>
            <tbody>
              ${ic.items.map((it) => `
                <tr>
                  <td><span class="badge ${it.abcClass === "A" ? "badge-pos" : it.abcClass === "B" ? "badge-info" : "badge-neutral"}">${it.abcClass}</span></td>
                  <td><strong>${esc(it.supplier)}</strong></td>
                  <td class="num">${fmtMoney(it.q4_2023)}</td>
                  <td class="num"><strong>${fmtMoney(it.q1_2024)}</strong></td>
                  <td class="num" style="color:${it.delta >= 0 ? "var(--positive)" : "var(--danger)"}">${it.delta >= 0 ? "+" : ""}${fmtMoney(it.delta)}</td>
                  <td class="num">${it.q4_2023 > 0 ? deltaBadge(it.deltaPct) : "—"}</td>
                  <td class="num">${it.shareQ1.toFixed(2).replace(".", ",")}%</td>
                  <td class="num">${it.cumShare.toFixed(1).replace(".", ",")}%</td>
                </tr>`).join("")}
            </tbody>
            <tfoot>
              <tr>
                <td colspan="2">ИТОГО отгрузки СМГ → ЧСТ</td>
                <td class="num">${fmtMoney(ic.totalQ4_23)}</td>
                <td class="num">${fmtMoney(ic.totalQ1_24)}</td>
                <td class="num">+${fmtMoney(ic.totalDelta)}</td>
                <td class="num">${deltaBadge(ic.growthPct)}</td>
                <td class="num">100,0%</td>
                <td class="num">100,0%</td>
              </tr>
            </tfoot>
          </table>
        </div>
      </div>
    `;

    window.SalesCharts.renderComboChart(document.getElementById("chart-ic-pareto"), {
      labels: ic.items.map((x) => x.supplier),
      unit: "BYN",
      rightUnit: "%",
      maxRight: 100,
      series: [
        { name: "4 кв. 2023", type: "bar", color: "#94a3b8", data: ic.items.map((x) => x.q4_2023) },
        { name: "1 кв. 2024", type: "bar", color: "#2563eb", data: ic.items.map((x) => x.q1_2024) },
        { name: "Накопленная доля %", type: "line", yAxis: "right", color: "#f59e0b", data: ic.items.map((x) => x.cumShare) }
      ]
    });

    const chinaActive = ic.chinaItems.filter((x) => x.q4_2023 > 0 || x.q1_2024 > 0);
    window.SalesCharts.renderComboChart(document.getElementById("chart-ic-china"), {
      labels: chinaActive.map((x) => x.supplier),
      unit: "BYN",
      series: [
        { name: "4 кв. 2023", type: "bar", color: "#94a3b8", data: chinaActive.map((x) => x.q4_2023) },
        { name: "1 кв. 2024", type: "bar", color: "#10b981", data: chinaActive.map((x) => x.q1_2024) }
      ]
    });
  }

  // ==================== 5. WAREHOUSES & CHANNELS ====================
  function renderWarehouses(el, rep) {
    const wd = rep.warehouses;
    const filtered = wd.items.filter((x) => state.warehouseChannel === "all" || x.channel === state.warehouseChannel);

    el.innerHTML = `
      <div class="kpi-grid">
        ${wd.channelSummary.map((ch) => `
          <div class="kpi-card kpi-info">
            <div class="kpi-label">${esc(ch.channel)}</div>
            <div class="kpi-value">${fmtCompact(ch.stockEnd, "BYN")}</div>
            <div class="kpi-meta">
              <span>Доля ${ch.shareEnd.toFixed(1).replace(".", ",")}% · было ${fmtCompact(ch.stockStart, "")}</span>
              ${deltaBadge(ch.deltaPct, true)}
            </div>
          </div>`).join("")}
      </div>

      <div class="grid-2">
        <div class="panel">
          <div class="panel-header">
            <div>
              <div class="panel-title">Структура складских запасов по каналам (${esc(wd.dateEnd)})</div>
              <div class="panel-subtitle">Соотношение центральных складов, розницы, B2B-филиалов и сервиса</div>
            </div>
          </div>
          <div class="panel-body">
            <div class="chart-wrap"><canvas id="chart-wh-donut" class="chart-canvas"></canvas></div>
          </div>
        </div>

        <div class="panel">
          <div class="panel-header">
            <div>
              <div class="panel-title">Изменение запасов по складам (${esc(wd.dateStart)} → ${esc(wd.dateEnd)})</div>
              <div class="panel-subtitle">Перераспределение стока из Уручья (-385 тыс.) в ГС Малиновка (+940 тыс.)</div>
            </div>
          </div>
          <div class="panel-body">
            <div class="chart-wrap"><canvas id="chart-wh-delta" class="chart-canvas"></canvas></div>
          </div>
        </div>
      </div>

      <div class="panel">
        <div class="panel-header">
          <div>
            <div class="panel-title">Детализация остатков по складам и региональным отделам продаж</div>
            <div class="panel-subtitle">По себестоимости, бел. руб. без НДС</div>
          </div>
          <div class="segmented" id="seg-wh-channel">
            <button class="seg-btn ${state.warehouseChannel === "all" ? "active" : ""}" data-val="all">Все</button>
            <button class="seg-btn ${state.warehouseChannel === "Центральные склады" ? "active" : ""}" data-val="Центральные склады">Центр. склады</button>
            <button class="seg-btn ${state.warehouseChannel === "Розничные магазины" ? "active" : ""}" data-val="Розничные магазины">Розница</button>
            <button class="seg-btn ${state.warehouseChannel === "Отделы продаж B2B" ? "active" : ""}" data-val="Отделы продаж B2B">Отделы продаж B2B</button>
            <button class="seg-btn ${state.warehouseChannel === "Сервис и производство" ? "active" : ""}" data-val="Сервис и производство">Сервис</button>
          </div>
        </div>
        <div class="table-scroll">
          <table class="data-table">
            <thead>
              <tr>
                <th>Склад / Филиал</th>
                <th>Канал</th>
                <th class="num">На ${esc(wd.dateStart)}, BYN</th>
                <th class="num">На ${esc(wd.dateEnd)}, BYN</th>
                <th class="num">Изменение, BYN</th>
                <th class="num">Динамика, %</th>
                <th class="num">Доля в запасах</th>
              </tr>
            </thead>
            <tbody>
              ${filtered.map((w) => `
                <tr>
                  <td><strong>${esc(w.name)}</strong></td>
                  <td><span class="badge badge-neutral">${esc(w.channel)}</span></td>
                  <td class="num">${fmtMoney(w.stockStart)}</td>
                  <td class="num"><strong>${fmtMoney(w.stockEnd)}</strong></td>
                  <td class="num" style="color:${w.delta >= 0 ? "var(--text-primary)" : "var(--positive)"}">${w.delta >= 0 ? "+" : ""}${fmtMoney(w.delta)}</td>
                  <td class="num">${deltaBadge(w.deltaPct, true)}</td>
                  <td class="num">${w.shareEnd.toFixed(1).replace(".", ",")}%</td>
                </tr>`).join("")}
            </tbody>
          </table>
        </div>
      </div>

      ${wd.rbServiceRows.length > 0 ? `
      <div class="panel">
        <div class="panel-header">
          <div>
            <div class="panel-title">Подразделения РБ: Сервис, Инженеры и Производственный отдел (2023 vs 2024)</div>
            <div class="panel-subtitle">Данные листа «РБ» — кратный рост объёма производственного отдела (+93,0%)</div>
          </div>
        </div>
        <div class="table-scroll">
          <table class="data-table">
            <thead>
              <tr><th>Подразделение / Инженер</th><th class="num">2023 г., BYN</th><th class="num">2024 г., BYN</th><th class="num">Изменение, BYN</th><th class="num">Динамика</th></tr>
            </thead>
            <tbody>
              ${wd.rbServiceRows.map((r) => `
                <tr>
                  <td>${esc(r.unit)}</td>
                  <td class="num">${fmtMoney(r.val2023)}</td>
                  <td class="num"><strong>${fmtMoney(r.val2024)}</strong></td>
                  <td class="num">${r.delta >= 0 ? "+" : ""}${fmtMoney(r.delta)}</td>
                  <td class="num">${r.val2023 > 0 ? deltaBadge(r.deltaPct) : "—"}</td>
                </tr>`).join("")}
            </tbody>
          </table>
        </div>
      </div>` : ""}
    `;

    el.querySelectorAll("#seg-wh-channel .seg-btn").forEach((btn) => {
      btn.onclick = () => {
        state.warehouseChannel = btn.dataset.val;
        renderWarehouses(el, rep);
      };
    });

    window.SalesCharts.renderDonutChart(document.getElementById("chart-wh-donut"), {
      centerLabel: "СКЛАДЫ",
      unit: "BYN",
      items: wd.channelSummary.map((c) => ({ label: c.channel, value: c.stockEnd }))
    });

    window.SalesCharts.renderHorizBarChart(document.getElementById("chart-wh-delta"), {
      unit: "BYN",
      items: wd.items.slice().sort((a, b) => b.delta - a.delta).map((x) => ({
        label: x.name,
        value: x.delta,
        sub: `${wd.dateEnd}: ${fmtMoney(x.stockEnd, "BYN")}`
      }))
    });
  }

  // ==================== 6. GROUPS & ABC ====================
  function renderGroups(el, rep) {
    const gd = rep.groups;
    const q = state.groupsQuery.trim().toLowerCase();
    const filtered = gd.items.filter((g) => {
      if (state.groupsABC !== "all" && g.abcClass !== state.groupsABC) return false;
      if (q && !g.supplier.toLowerCase().includes(q)) return false;
      return true;
    });

    el.innerHTML = `
      <div class="kpi-grid">
        <div class="kpi-card kpi-info">
          <div class="kpi-label">Остатки по 20 группам (30.06.2024)</div>
          <div class="kpi-value">${fmtCompact(gd.trackedEnd, "BYN")}</div>
          <div class="kpi-meta"><span>Охват ${gd.coverageEndPct.toFixed(1).replace(".", ",")}% склада компании</span>${deltaBadge(gd.trackedGrowthPct, true)}</div>
        </div>
        <div class="kpi-card kpi-warning">
          <div class="kpi-label">Прирост в группах за полугодие</div>
          <div class="kpi-value">+${fmtCompact(gd.trackedDelta, "BYN")}</div>
          <div class="kpi-meta"><span>31.12.2023: ${fmtCompact(gd.trackedStart, "BYN")}</span></div>
        </div>
        <div class="kpi-card kpi-positive">
          <div class="kpi-label">Группа A (80% запасов)</div>
          <div class="kpi-value">${gd.items.filter((x) => x.abcClass === "A").length} групп</div>
          <div class="kpi-meta"><span>Керхер Проф + H&G = 45,3% запасов</span></div>
        </div>
        <div class="kpi-card kpi-info">
          <div class="kpi-label">Поставщики РФ на складе (лист РФ)</div>
          <div class="kpi-value">${fmtCompact(gd.rfSupplierStocks.reduce((a, b) => a + b.stock2403, 0), "BYN")}</div>
          <div class="kpi-meta"><span>9 ключевых поставщиков РФ</span></div>
        </div>
      </div>

      <div class="panel">
        <div class="panel-header">
          <div>
            <div class="panel-title">ABC-кривая Парето по товарным группам (30.06.2024 vs 31.12.2023)</div>
            <div class="panel-subtitle">Объём товарного запаса по брендам/поставщикам (BYN) и кумулятивная доля капитала (%)</div>
          </div>
        </div>
        <div class="panel-body">
          <div class="chart-wrap chart-wrap-tall"><canvas id="chart-groups-abc" class="chart-canvas"></canvas></div>
        </div>
      </div>

      <div class="panel">
        <div class="panel-header">
          <div>
            <div class="panel-title">Матрица товарных групп (лист «Группы»)</div>
            <div class="panel-subtitle">Фильтрация по ABC-классу и поиск по бренду/поставщику</div>
          </div>
          <div class="filter-bar">
            <input type="text" class="input-field" id="inp-groups-q" placeholder="Поиск группы..." value="${esc(state.groupsQuery)}">
            <div class="segmented" id="seg-groups-abc">
              <button class="seg-btn ${state.groupsABC === "all" ? "active" : ""}" data-val="all">Все (${gd.items.length})</button>
              <button class="seg-btn ${state.groupsABC === "A" ? "active" : ""}" data-val="A">A</button>
              <button class="seg-btn ${state.groupsABC === "B" ? "active" : ""}" data-val="B">B</button>
              <button class="seg-btn ${state.groupsABC === "C" ? "active" : ""}" data-val="C">C</button>
            </div>
          </div>
        </div>
        <div class="table-scroll">
          <table class="data-table">
            <thead>
              <tr>
                <th>ABC</th>
                <th>Поставщик / Товарная группа</th>
                <th class="num">Остатки 31.12.2023, BYN</th>
                <th class="num">Остатки 30.06.2024, BYN</th>
                <th class="num">Разница, BYN</th>
                <th class="num">Разница, %</th>
                <th class="num">Доля</th>
                <th class="num">Накопл. %</th>
              </tr>
            </thead>
            <tbody>
              ${filtered.map((g) => `
                <tr>
                  <td><span class="badge ${g.abcClass === "A" ? "badge-pos" : g.abcClass === "B" ? "badge-info" : "badge-neutral"}">${g.abcClass}</span></td>
                  <td><strong>${esc(g.supplier)}</strong></td>
                  <td class="num">${fmtMoney(g.stock2023)}</td>
                  <td class="num"><strong>${fmtMoney(g.stock2024)}</strong></td>
                  <td class="num" style="color:${g.delta > 0 ? "var(--warning)" : "var(--positive)"}">${g.delta > 0 ? "+" : ""}${fmtMoney(g.delta)}</td>
                  <td class="num">${deltaBadge(g.deltaPct, true)}</td>
                  <td class="num">
                    <span class="bar-inline"><span class="bar-inline-fill" style="width:${Math.min(100, g.sharePct * 3.2)}%"></span></span>
                    ${g.sharePct.toFixed(1).replace(".", ",")}%
                  </td>
                  <td class="num">${g.cumShare.toFixed(1).replace(".", ",")}%</td>
                </tr>`).join("")}
            </tbody>
          </table>
        </div>
      </div>

      ${gd.rfSupplierStocks.length > 0 ? `
      <div class="panel">
        <div class="panel-header">
          <div>
            <div class="panel-title">Детализация остатков российских поставщиков на складе РБ (лист «РФ»)</div>
            <div class="panel-subtitle">Снижение общих остатков по поставщикам РФ на -14,9% (-36,6 тыс. BYN)</div>
          </div>
        </div>
        <div class="table-scroll">
          <table class="data-table">
            <thead>
              <tr><th>Поставщик РФ</th><th class="num">31.12.2023, BYN</th><th class="num">31.03.2024, BYN</th><th class="num">Разница, BYN</th><th class="num">Динамика</th></tr>
            </thead>
            <tbody>
              ${gd.rfSupplierStocks.map((s) => `
                <tr>
                  <td><strong>${esc(s.supplier)}</strong></td>
                  <td class="num">${fmtMoney(s.stock2312)}</td>
                  <td class="num"><strong>${fmtMoney(s.stock2403)}</strong></td>
                  <td class="num">${s.delta >= 0 ? "+" : ""}${fmtMoney(s.delta)}</td>
                  <td class="num">${deltaBadge(s.deltaPct, true)}</td>
                </tr>`).join("")}
            </tbody>
          </table>
        </div>
      </div>` : ""}
    `;

    document.getElementById("inp-groups-q").oninput = (e) => {
      state.groupsQuery = e.target.value;
      const pos = e.target.selectionStart;
      renderGroups(el, rep);
      const inp = document.getElementById("inp-groups-q");
      inp.focus();
      inp.setSelectionRange(pos, pos);
    };

    el.querySelectorAll("#seg-groups-abc .seg-btn").forEach((btn) => {
      btn.onclick = () => {
        state.groupsABC = btn.dataset.val;
        renderGroups(el, rep);
      };
    });

    window.SalesCharts.renderComboChart(document.getElementById("chart-groups-abc"), {
      labels: gd.items.map((g) => g.supplier),
      unit: "BYN",
      rightUnit: "%",
      maxRight: 100,
      series: [
        { name: "31.12.2023", type: "bar", color: "#94a3b8", data: gd.items.map((g) => g.stock2023) },
        { name: "30.06.2024", type: "bar", color: "#2563eb", data: gd.items.map((g) => g.stock2024) },
        { name: "Кумулятивная доля %", type: "line", yAxis: "right", color: "#f59e0b", data: gd.items.map((g) => g.cumShare) }
      ]
    });
  }

  // ==================== 7. RF PROCUREMENT & 1C SKU ====================
  function renderRF(el, rep) {
    const rf = rep.rfProcurement;
    const q = state.rfQuery.trim().toLowerCase();
    const filteredItems = rf.lineItems.filter((li) => {
      if (state.rfCategory !== "all" && li.category !== state.rfCategory) return false;
      if (state.rfMonth !== "all" && li.month !== state.rfMonth) return false;
      if (q && !(li.sku.toLowerCase().includes(q) || li.supplier.toLowerCase().includes(q) || li.docNumber.toLowerCase().includes(q))) {
        return false;
      }
      return true;
    });

    const pageSize = 50;
    const totalPages = Math.max(1, Math.ceil(filteredItems.length / pageSize));
    if (state.rfPage > totalPages) state.rfPage = totalPages;
    const slice = filteredItems.slice((state.rfPage - 1) * pageSize, state.rfPage * pageSize);

    el.innerHTML = `
      <div class="kpi-grid">
        <div class="kpi-card kpi-positive">
          <div class="kpi-label">Внешние закупки РФ-РФ (1 кв. 2024)</div>
          <div class="kpi-value">${fmtCompact(rf.externalQ1_2024, "RUB")}</div>
          <div class="kpi-meta"><span>4 кв. 2023: ${fmtCompact(rf.externalQ4_2023, "RUB")}</span>${deltaBadge(rf.externalGrowthPct)}</div>
        </div>
        <div class="kpi-card kpi-info">
          <div class="kpi-label">Полный оборот 1С (с Керхер и СМГ)</div>
          <div class="kpi-value">${fmtCompact(rf.total1CWithVAT, "RUB")}</div>
          <div class="kpi-meta"><span>Без НДС: ${fmtCompact(rf.total1CNoVAT, "RUB")}</span></div>
        </div>
        <div class="kpi-card kpi-info">
          <div class="kpi-label">Контрагентов / Документов 1С</div>
          <div class="kpi-value">${rf.suppliers1C.length} / ${rf.documentCount}</div>
          <div class="kpi-meta"><span>Период: ${esc(rf.period1C || "01.01.2024 – 31.03.2024")}</span></div>
        </div>
        <div class="kpi-card kpi-positive">
          <div class="kpi-label">Товарных строк SKU в выгрузке</div>
          <div class="kpi-value">${rf.lineItemCount}</div>
          <div class="kpi-meta"><span>Суммарно ед.: ${fmtMoney(rf.total1CQty)}</span></div>
        </div>
      </div>

      <div class="grid-2">
        <div class="panel">
          <div class="panel-header">
            <div>
              <div class="panel-title">Внешние закупки РФ-РФ: 4 кв. 2023 vs 1 кв. 2024 (RUB с НДС)</div>
              <div class="panel-subtitle">Крупнейшие внешние российские поставщики (лист «4-ый квартал 2023 (сравнение)»)</div>
            </div>
          </div>
          <div class="panel-body">
            <div class="chart-wrap"><canvas id="chart-rf-comp" class="chart-canvas"></canvas></div>
          </div>
        </div>

        <div class="panel">
          <div class="panel-header">
            <div>
              <div class="panel-title">Помесячная динамика поступлений по документам 1С (1 кв. 2024)</div>
              <div class="panel-subtitle">Разделение на внешние закупки РФ и поставки Керхер ООО / СМГ ООО</div>
            </div>
          </div>
          <div class="panel-body">
            <div class="chart-wrap"><canvas id="chart-rf-months" class="chart-canvas"></canvas></div>
          </div>
        </div>
      </div>

      <div class="panel">
        <div class="panel-header">
          <div>
            <div class="panel-title">Реестр поставщиков 1С (лист «Закупки РФ-РФ»)</div>
            <div class="panel-subtitle">33 контрагента с количеством документов, номенклатурных позиций и суммами с НДС</div>
          </div>
        </div>
        <div class="table-scroll" style="max-height:340px;overflow-y:auto">
          <table class="data-table">
            <thead>
              <tr>
                <th>Поставщик</th>
                <th>Категория</th>
                <th class="num">Док-тов</th>
                <th class="num">Строк SKU</th>
                <th class="num">Кол-во</th>
                <th class="num">Без НДС, RUB</th>
                <th class="num">Сумма с НДС, RUB</th>
                <th class="num">Доля</th>
              </tr>
            </thead>
            <tbody>
              ${rf.suppliers1C.map((s) => `
                <tr>
                  <td><strong>${esc(s.supplier)}</strong></td>
                  <td><span class="badge badge-neutral">${esc(s.category)}</span></td>
                  <td class="num">${s.docCount}</td>
                  <td class="num">${s.itemCount}</td>
                  <td class="num">${fmtMoney(s.qty)}</td>
                  <td class="num">${fmtMoney(s.sumNoVAT)}</td>
                  <td class="num"><strong>${fmtMoney(s.sumWithVAT)}</strong></td>
                  <td class="num">${s.sharePct.toFixed(2).replace(".", ",")}%</td>
                </tr>`).join("")}
            </tbody>
          </table>
        </div>
      </div>

      <div class="panel">
        <div class="panel-header">
          <div>
            <div class="panel-title">Поиск по номенклатуре (SKU) и документам 1С (${filteredItems.length} поз.)</div>
            <div class="panel-subtitle">Сумма отфильтрованных позиций: <strong>${fmtMoney(filteredItems.reduce((a, b) => a + b.sumWithVAT, 0), "RUB с НДС")}</strong></div>
          </div>
          <div class="filter-bar">
            <input type="text" class="input-field" id="inp-rf-q" placeholder="Поиск по SKU, поставщику, № документа..." value="${esc(state.rfQuery)}" style="min-width:250px">
            <select class="select-field" id="sel-rf-cat">
              <option value="all" ${state.rfCategory === "all" ? "selected" : ""}>Все категории</option>
              <option value="Основные внешние (+)" ${state.rfCategory === "Основные внешние (+)" ? "selected" : ""}>Основные внешние (+)</option>
              <option value="Керхер / Внутригрупповые (-)" ${state.rfCategory === "Керхер / Внутригрупповые (-)" ? "selected" : ""}>Керхер / СМГ (-)</option>
              <option value="Закупки сервис" ${state.rfCategory === "Закупки сервис" ? "selected" : ""}>Закупки сервис</option>
              <option value="Разовые закупки" ${state.rfCategory === "Разовые закупки" ? "selected" : ""}>Разовые закупки</option>
            </select>
            <select class="select-field" id="sel-rf-month">
              <option value="all" ${state.rfMonth === "all" ? "selected" : ""}>Весь 1 кв. 2024</option>
              <option value="Январь 2024" ${state.rfMonth === "Январь 2024" ? "selected" : ""}>Январь 2024</option>
              <option value="Февраль 2024" ${state.rfMonth === "Февраль 2024" ? "selected" : ""}>Февраль 2024</option>
              <option value="Март 2024" ${state.rfMonth === "Март 2024" ? "selected" : ""}>Март 2024</option>
            </select>
          </div>
        </div>
        <div class="table-scroll">
          <table class="data-table">
            <thead>
              <tr>
                <th>Дата</th>
                <th>Документ</th>
                <th>Поставщик</th>
                <th>Номенклатура (SKU)</th>
                <th class="num">Кол-во</th>
                <th class="num">НДС</th>
                <th class="num">Сумма с НДС, RUB</th>
              </tr>
            </thead>
            <tbody>
              ${slice.map((li) => `
                <tr>
                  <td>${esc(li.docDate)}</td>
                  <td><span class="badge badge-neutral">${esc(li.docNumber)}</span></td>
                  <td>${esc(li.supplier)}</td>
                  <td><strong>${esc(li.sku)}</strong></td>
                  <td class="num">${fmtMoney(li.qty)}</td>
                  <td class="num">${esc(li.vatRate)}</td>
                  <td class="num"><strong>${fmtMoney(li.sumWithVAT)}</strong></td>
                </tr>`).join("")}
            </tbody>
          </table>
        </div>
        <div class="panel-header" style="border-top:1px solid var(--border-color);border-bottom:none">
          <div style="font-size:12px;color:var(--text-muted)">Страница ${state.rfPage} из ${totalPages} (показано ${slice.length} из ${filteredItems.length})</div>
          <div class="filter-bar">
            <button class="btn btn-sm" id="btn-rf-prev" ${state.rfPage <= 1 ? "disabled" : ""}>← Назад</button>
            <button class="btn btn-sm" id="btn-rf-next" ${state.rfPage >= totalPages ? "disabled" : ""}>Вперёд →</button>
          </div>
        </div>
      </div>
    `;

    document.getElementById("inp-rf-q").oninput = (e) => {
      state.rfQuery = e.target.value;
      state.rfPage = 1;
      const pos = e.target.selectionStart;
      renderRF(el, rep);
      const inp = document.getElementById("inp-rf-q");
      inp.focus();
      inp.setSelectionRange(pos, pos);
    };
    document.getElementById("sel-rf-cat").onchange = (e) => {
      state.rfCategory = e.target.value;
      state.rfPage = 1;
      renderRF(el, rep);
    };
    document.getElementById("sel-rf-month").onchange = (e) => {
      state.rfMonth = e.target.value;
      state.rfPage = 1;
      renderRF(el, rep);
    };
    document.getElementById("btn-rf-prev").onclick = () => {
      if (state.rfPage > 1) { state.rfPage--; renderRF(el, rep); }
    };
    document.getElementById("btn-rf-next").onclick = () => {
      if (state.rfPage < totalPages) { state.rfPage++; renderRF(el, rep); }
    };

    const topComp = rf.comparisonRows.slice().sort((a, b) => (b.q1_2024 + b.q4_2023) - (a.q1_2024 + a.q4_2023)).slice(0, 10);
    window.SalesCharts.renderComboChart(document.getElementById("chart-rf-comp"), {
      labels: topComp.map((c) => c.supplier),
      unit: "RUB",
      series: [
        { name: "4 кв. 2023", type: "bar", color: "#94a3b8", data: topComp.map((c) => c.q4_2023) },
        { name: "1 кв. 2024", type: "bar", color: "#2563eb", data: topComp.map((c) => c.q1_2024) }
      ]
    });

    window.SalesCharts.renderComboChart(document.getElementById("chart-rf-months"), {
      labels: rf.monthly1C.map((m) => m.month),
      stacked: true,
      unit: "RUB",
      series: [
        { name: "Внешние поставщики РФ", type: "bar", color: "#10b981", data: rf.monthly1C.map((m) => m.externalRUB) },
        { name: "Керхер ООО и СМГ ООО", type: "bar", color: "#2563eb", data: rf.monthly1C.map((m) => m.internalRUB) }
      ]
    });
  }

  // ==================== 8. PREPAYMENTS & FX SIMULATOR ====================
  function renderPrepayments(el, rep) {
    const pd = rep.prepayments;
    const rateFor = (code) => state.fxRates[code] || state.baseFxRates[code] || 1;

    const recalcList = (list) => list.map((it) => {
      const r = rateFor(it.currencyCode);
      const purch = it.amountOrig * r;
      const cust = it.customsOrig * r;
      return Object.assign({}, it, {
        simPurchaseBYN: purch,
        simCustomsBYN: cust,
        simTotalBYN: purch + cust
      });
    });

    const simPaid = recalcList(pd.paidOrders);
    const simUnpaid = recalcList(pd.unpaidOrders);
    const simPaidTot = simPaid.reduce((a, b) => a + b.simTotalBYN, 0);
    const simUnpaidTot = simUnpaid.reduce((a, b) => a + b.simTotalBYN, 0);
    const simDebtBYN = pd.karcherRFDebtRUB * rateFor("RUB");
    const simGrand = simPaidTot + simUnpaidTot + simDebtBYN;
    const simDiff = simGrand - pd.grandTotalBYN;

    el.innerHTML = `
      <div class="kpi-grid">
        <div class="kpi-card kpi-info">
          <div class="kpi-label">Уже оплачено + таможня</div>
          <div class="kpi-value">${fmtCompact(simPaidTot, "BYN")}</div>
          <div class="kpi-meta"><span>База документа: ${fmtCompact(pd.paidTotalBYN, "BYN")}</span></div>
        </div>
        <div class="kpi-card kpi-warning">
          <div class="kpi-label">Необходимо оплатить + таможня</div>
          <div class="kpi-value">${fmtCompact(simUnpaidTot, "BYN")}</div>
          <div class="kpi-meta"><span>База документа: ${fmtCompact(pd.unpaidTotalBYN, "BYN")}</span></div>
        </div>
        <div class="kpi-card kpi-danger">
          <div class="kpi-label">Задолженность Керхер РФ</div>
          <div class="kpi-value">${fmtCompact(simDebtBYN, "BYN")}</div>
          <div class="kpi-meta"><span>${fmtCompact(pd.karcherRFDebtRUB, "RUB")}</span></div>
        </div>
        <div class="kpi-card ${Math.abs(simDiff) > 50 ? "kpi-warning" : "kpi-positive"}">
          <div class="kpi-label">ИТОГО обязательств (сценарий)</div>
          <div class="kpi-value">${fmtCompact(simGrand, "BYN")}</div>
          <div class="kpi-meta">
            <span>Отклонение от базы: ${simDiff >= 0 ? "+" : ""}${fmtMoney(simDiff, "BYN")}</span>
          </div>
        </div>
      </div>

      <div class="grid-65-35">
        <div class="panel">
          <div class="panel-header">
            <div>
              <div class="panel-title">Валютный калькулятор и стресс-тест курсов</div>
              <div class="panel-subtitle">Изменяйте курсы валют для мгновенного пересчёта таможни и сумм к оплате в BYN</div>
            </div>
            <div class="filter-bar">
              <button class="btn btn-sm" data-fx-mult="1">Курс из документа</button>
              <button class="btn btn-sm" data-fx-mult="1.05">+5% к курсам</button>
              <button class="btn btn-sm" data-fx-mult="1.10">+10% стресс</button>
              <button class="btn btn-sm" data-fx-mult="0.95">-5% укрепление</button>
            </div>
          </div>
          <div class="panel-body">
            <div class="fx-controls-grid">
              ${[
                { code: "EUR", title: "1 EUR (Евро)", scale: 1 },
                { code: "CNY", title: "10 CNY (Юаней)", scale: 10 },
                { code: "RUB", title: "100 RUB (Росс. руб.)", scale: 100 },
                { code: "PLN", title: "10 PLN (Злотых)", scale: 10 },
                { code: "USD", title: "1 USD (Доллар)", scale: 1 }
              ].map((fx) => {
                const curScaled = (rateFor(fx.code) * fx.scale).toFixed(4);
                const baseScaled = state.baseFxRates[fx.code] * fx.scale;
                return `
                  <div class="fx-box">
                    <label><span>${fx.title}</span><span style="color:var(--text-muted)">база ${baseScaled.toFixed(4)}</span></label>
                    <input type="number" step="0.01" class="fx-num-inp" data-code="${fx.code}" data-scale="${fx.scale}" value="${curScaled}">
                  </div>`;
              }).join("")}
            </div>
          </div>
        </div>

        <div class="panel">
          <div class="panel-header">
            <div>
              <div class="panel-title">Валютная структура обязательств</div>
              <div class="panel-subtitle">Распределение суммы 2,75 млн BYN по валютам контрактов</div>
            </div>
          </div>
          <div class="panel-body">
            <div class="chart-wrap chart-wrap-sm"><canvas id="chart-fx-donut" class="chart-canvas"></canvas></div>
          </div>
        </div>
      </div>

      <div class="panel">
        <div class="panel-header">
          <div>
            <div class="panel-title">Реестр предварительных оплат и заказов к оплате</div>
            <div class="panel-subtitle">Пересчёт закупочной стоимости и таможенных платежей (ТП + НДС) по выбранным курсам</div>
          </div>
        </div>
        <div class="table-scroll">
          <table class="data-table">
            <thead>
              <tr>
                <th>Статус</th>
                <th>Поставщик / Товарная позиция</th>
                <th class="num">Сумма в валюте</th>
                <th>Валюта</th>
                <th class="num">Таможня в вал.</th>
                <th class="num">Закупка, BYN</th>
                <th class="num">Таможня, BYN</th>
                <th class="num">Итого с таможней, BYN</th>
              </tr>
            </thead>
            <tbody>
              ${simPaid.concat(simUnpaid).map((p) => `
                <tr>
                  <td><span class="badge ${p.status === "paid" ? "badge-pos" : "badge-warn"}">${p.status === "paid" ? "Оплачено" : "К оплате"}</span></td>
                  <td><strong>${esc(p.supplier)}</strong></td>
                  <td class="num">${fmtMoney(p.amountOrig)}</td>
                  <td>${esc(p.currency)}</td>
                  <td class="num">${fmtMoney(p.customsOrig)}</td>
                  <td class="num">${fmtMoney(p.simPurchaseBYN)}</td>
                  <td class="num">${fmtMoney(p.simCustomsBYN)}</td>
                  <td class="num"><strong>${fmtMoney(p.simTotalBYN)}</strong></td>
                </tr>`).join("")}
              ${pd.karcherRFDebtRUB > 0 ? `
                <tr>
                  <td><span class="badge badge-neg">Задолженность</span></td>
                  <td><strong>Керхер РФ</strong></td>
                  <td class="num">${fmtMoney(pd.karcherRFDebtRUB)}</td>
                  <td>росс.руб</td>
                  <td class="num">0</td>
                  <td class="num">${fmtMoney(simDebtBYN)}</td>
                  <td class="num">0</td>
                  <td class="num"><strong>${fmtMoney(simDebtBYN)}</strong></td>
                </tr>` : ""}
            </tbody>
            <tfoot>
              <tr>
                <td colspan="5">ИТОГО обязательств с таможней</td>
                <td class="num">${fmtMoney(simPaid.concat(simUnpaid).reduce((a, b) => a + b.simPurchaseBYN, 0) + simDebtBYN)}</td>
                <td class="num">${fmtMoney(simPaid.concat(simUnpaid).reduce((a, b) => a + b.simCustomsBYN, 0))}</td>
                <td class="num">${fmtMoney(simGrand)}</td>
              </tr>
            </tfoot>
          </table>
        </div>
      </div>
    `;

    el.querySelectorAll("[data-fx-mult]").forEach((btn) => {
      btn.onclick = () => {
        const mult = parseFloat(btn.dataset.fxMult);
        Object.keys(state.baseFxRates).forEach((k) => {
          state.fxRates[k] = state.baseFxRates[k] * mult;
        });
        renderPrepayments(el, rep);
      };
    });

    el.querySelectorAll(".fx-num-inp").forEach((inp) => {
      inp.onchange = () => {
        const v = parseFloat(inp.value);
        const sc = parseFloat(inp.dataset.scale || "1");
        if (v > 0 && sc > 0) {
          state.fxRates[inp.dataset.code] = v / sc;
          renderPrepayments(el, rep);
        }
      };
    });

    window.SalesCharts.renderDonutChart(document.getElementById("chart-fx-donut"), {
      centerLabel: "ВАЛЮТЫ",
      unit: "BYN",
      items: pd.currencyExposure.map((c) => ({ label: c.currencyCode, value: c.totalBYN }))
    });
  }

  // ==================== 9. ALL RAW SHEETS ====================
  function renderSheets(el, rep) {
    const sheets = rep.rawSheets || [];
    if (!sheets.length) {
      el.innerHTML = `<div class="panel"><div class="panel-body">В книге нет листов</div></div>`;
      return;
    }
    const idx = Math.min(state.selectedSheetIdx, sheets.length - 1);
    const sh = sheets[idx];
    const q = state.sheetQuery.trim().toLowerCase();

    const visibleRows = sh.rows.filter((row) => {
      if (!q) return true;
      return row.some((c) => c && c.t && c.t.toLowerCase().includes(q));
    });

    const colCount = sh.rows.reduce((m, r) => Math.max(m, r.length), 0);
    const colLetters = [];
    for (let c = 1; c <= colCount; c++) {
      let n = c, s = "";
      while (n > 0) { n--; s = String.fromCharCode(65 + (n % 26)) + s; n = Math.floor(n / 26); }
      colLetters.push(s);
    }

    el.innerHTML = `
      <div class="panel">
        <div class="panel-header">
          <div class="filter-bar">
            <label style="font-weight:700">Лист Excel:</label>
            <select class="select-field" id="sel-raw-sheet">
              ${sheets.map((s, i) => `<option value="${i}" ${i === idx ? "selected" : ""}>${i + 1}. ${esc(s.name)} (${s.maxRow} стр.)</option>`).join("")}
            </select>
            <input type="text" class="input-field" id="inp-sheet-q" placeholder="Фильтр строк на листе..." value="${esc(state.sheetQuery)}">
          </div>
          <div style="font-size:12px;color:var(--text-muted)">
            Синяя точка <span class="formula-dot"></span> означает ячейку с формулой Excel (наведите курсор для просмотра формулы)
          </div>
        </div>
        <div class="table-scroll" style="max-height:640px;overflow-y:auto">
          <table class="data-table">
            <thead>
              <tr>
                <th style="width:46px">#</th>
                ${colLetters.map((l) => `<th>${l}</th>`).join("")}
              </tr>
            </thead>
            <tbody>
              ${visibleRows.map((row, rIdx) => `
                <tr>
                  <td style="color:var(--text-muted);font-size:11px">${rIdx + 1}</td>
                  ${colLetters.map((_, cIdx) => {
                    const cell = row[cIdx] || {};
                    const pad = cell.i ? `padding-left:${12 + cell.i * 8}px;` : "";
                    const title = cell.f ? `title="Формула: =${esc(cell.f)}"` : "";
                    return `<td class="${cell.n ? "num" : ""} ${cell.f ? "formula-cell" : ""}" style="${pad}" ${title}>
                      ${esc(cell.t || "")}${cell.f ? `<span class="formula-dot"></span>` : ""}
                    </td>`;
                  }).join("")}
                </tr>`).join("")}
            </tbody>
          </table>
        </div>
        ${sh.truncated ? `<div class="panel-body" style="color:var(--text-muted);font-size:12px">Показаны первые 500 строк из ${sh.maxRow}. Для полной работы со всеми строками используйте вкладку «Закупки РФ-РФ (1С SKU)» или кнопку «Экспорт в Excel (CSV)».</div>` : ""}
      </div>
    `;

    document.getElementById("sel-raw-sheet").onchange = (e) => {
      state.selectedSheetIdx = parseInt(e.target.value, 10) || 0;
      renderSheets(el, rep);
    };
    document.getElementById("inp-sheet-q").oninput = (e) => {
      state.sheetQuery = e.target.value;
      const pos = e.target.selectionStart;
      renderSheets(el, rep);
      const inp = document.getElementById("inp-sheet-q");
      inp.focus();
      inp.setSelectionRange(pos, pos);
    };
  }

  // ==================== FALLBACK FOR ARBITRARY CUSTOM EXCEL ====================
  function renderGenericWorkbook(el, rep) {
    const gt = rep.genericTables || [];
    el.innerHTML = `
      <div class="panel">
        <div class="panel-header">
          <div>
            <div class="panel-title">Автоматический анализ загруженной книги Excel: ${esc(rep.fileName)}</div>
            <div class="panel-subtitle">Найдено листов: ${rep.sheetCount} · Перейдите в раздел «Все листы Excel» для построчного просмотра</div>
          </div>
          <button class="btn btn-primary btn-sm" id="btn-goto-sheets">Открыть таблицы листов</button>
        </div>
        <div class="panel-body">
          ${gt.length === 0 ? `<div>Числовые аналитические колонки не обнаружены. Откройте вкладку «Все листы Excel».</div>` :
            gt.map((g, idx) => `
              <div style="margin-bottom:24px">
                <h3 style="margin-bottom:8px">${esc(g.sheetName)} (${g.rowCount} строк)</h3>
                <div class="table-scroll">
                  <table class="data-table">
                    <thead><tr><th>Числовой показатель</th><th class="num">Записей</th><th class="num">Сумма</th><th class="num">Среднее</th><th class="num">Мин</th><th class="num">Макс</th></tr></thead>
                    <tbody>
                      ${g.metricCols.map((m) => `
                        <tr>
                          <td><strong>${esc(m.header)}</strong></td>
                          <td class="num">${m.count}</td>
                          <td class="num"><strong>${fmtMoney(m.sum)}</strong></td>
                          <td class="num">${fmtMoney(m.avg)}</td>
                          <td class="num">${fmtMoney(m.min)}</td>
                          <td class="num">${fmtMoney(m.max)}</td>
                        </tr>`).join("")}
                    </tbody>
                  </table>
                </div>
                ${g.topPreview && g.topPreview.length > 0 ? `<div class="chart-wrap chart-wrap-sm" style="margin-top:12px"><canvas id="chart-gen-${idx}" class="chart-canvas"></canvas></div>` : ""}
              </div>`).join("")}
        </div>
      </div>
    `;
    const btnGo = document.getElementById("btn-goto-sheets");
    if (btnGo) btnGo.onclick = () => switchTab("sheets");

    gt.forEach((g, idx) => {
      if (g.topPreview && g.topPreview.length > 0) {
        window.SalesCharts.renderHorizBarChart(document.getElementById(`chart-gen-${idx}`), {
          items: g.topPreview.map((p) => ({ label: p.label, value: p.value }))
        });
      }
    });
  }

  // ==================== FILE UPLOAD, EXPORT & EVENTS ====================
  async function uploadExcelFile(file) {
    if (!file) return;
    const fd = new FormData();
    fd.append("file", file);
    showToast("Загрузка и анализ файла " + file.name + "...");
    try {
      const res = await fetch("api/upload", { method: "POST", body: fd });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "Ошибка чтения файла");
      applyPayload({ report: data.report, desktopMode: state.desktopMode, hasNativeDialog: state.hasNativeDialog });
      showToast("Файл успешно проанализирован: " + data.report.fileName);
    } catch (err) {
      showToast("Ошибка: " + err.message);
    }
  }

  async function openExcelAction() {
    if (state.hasNativeDialog) {
      try {
        const res = await fetch("api/open-dialog", { method: "POST" });
        const data = await res.json();
        if (data.supported) {
          if (data.cancelled) return;
          if (!res.ok) throw new Error(data.error || "Ошибка открытия файла");
          applyPayload({ report: data.report, desktopMode: state.desktopMode, hasNativeDialog: state.hasNativeDialog });
          showToast("Открыт файл: " + data.report.fileName);
          return;
        }
      } catch (_) {
        // Fallback to HTML input
      }
    }
    document.getElementById("file-input").click();
  }

  async function resetToDefault() {
    try {
      const res = await fetch("api/reset", { method: "POST" });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "Ошибка сброса");
      applyPayload({ report: data.report, desktopMode: state.desktopMode, hasNativeDialog: state.hasNativeDialog });
      showToast("Восстановлен базовый документ: " + data.report.fileName);
    } catch (err) {
      showToast("Ошибка: " + err.message);
    }
  }

  function exportCurrentCSV() {
    const meta = TAB_META[state.activeTab] || TAB_META.overview;
    let url = `api/export/csv?section=${encodeURIComponent(meta.csvSection)}`;
    if (state.activeTab === "sheets" && state.report && state.report.rawSheets[state.selectedSheetIdx]) {
      url += `&name=${encodeURIComponent(state.report.rawSheets[state.selectedSheetIdx].name)}`;
    }
    window.location.href = url;
  }

  function bindGlobalEvents() {
    document.querySelectorAll(".nav-btn").forEach((btn) => {
      btn.addEventListener("click", () => switchTab(btn.dataset.tab));
    });

    document.getElementById("btn-open-excel").onclick = openExcelAction;
    document.getElementById("btn-reset-excel").onclick = resetToDefault;
    document.getElementById("btn-export-csv").onclick = exportCurrentCSV;
    document.getElementById("btn-print").onclick = () => window.print();
    document.getElementById("btn-theme").onclick = toggleTheme;

    const fileInp = document.getElementById("file-input");
    fileInp.onchange = () => {
      if (fileInp.files && fileInp.files[0]) {
        uploadExcelFile(fileInp.files[0]);
        fileInp.value = "";
      }
    };

    // Drag & Drop .xlsx support anywhere in window
    const overlay = document.getElementById("drop-overlay");
    let dragDepth = 0;
    window.addEventListener("dragenter", (e) => {
      e.preventDefault();
      dragDepth++;
      overlay.classList.add("active");
    });
    window.addEventListener("dragover", (e) => e.preventDefault());
    window.addEventListener("dragleave", () => {
      dragDepth = Math.max(0, dragDepth - 1);
      if (dragDepth === 0) overlay.classList.remove("active");
    });
    window.addEventListener("drop", (e) => {
      e.preventDefault();
      dragDepth = 0;
      overlay.classList.remove("active");
      if (e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files[0]) {
        uploadExcelFile(e.dataTransfer.files[0]);
      }
    });

    let resizeTimer;
    window.addEventListener("resize", () => {
      clearTimeout(resizeTimer);
      resizeTimer = setTimeout(() => renderCurrentTab(), 120);
    });

    // Heartbeat for desktop app lifecycle
    setInterval(() => {
      fetch("api/ping", { method: "POST" }).catch(() => {});
    }, 4000);

    window.addEventListener("pagehide", () => {
      if (navigator.sendBeacon) {
        navigator.sendBeacon("api/window-closed");
      }
    });
  }

  initTheme();
  bindGlobalEvents();
  loadDashboard();
})();
