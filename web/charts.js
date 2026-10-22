// High-DPI interactive offline Canvas chart engine (zero external CDN dependencies)
(function () {
  const PALETTE = [
    "#2563eb", "#0ea5e9", "#10b981", "#f59e0b",
    "#8b5cf6", "#ef4444", "#14b8a6", "#ec4899",
    "#6366f1", "#84cc16"
  ];

  function getTooltip() {
    let el = document.getElementById("chart-tooltip");
    if (!el) {
      el = document.createElement("div");
      el.id = "chart-tooltip";
      el.className = "chart-tooltip";
      document.body.appendChild(el);
    }
    return el;
  }

  function showTooltip(ev, html) {
    const tt = getTooltip();
    tt.innerHTML = html;
    tt.style.display = "block";
    const pad = 14;
    let x = ev.clientX + pad;
    let y = ev.clientY + pad;
    const rect = tt.getBoundingClientRect();
    if (x + rect.width > window.innerWidth - 10) {
      x = ev.clientX - rect.width - pad;
    }
    if (y + rect.height > window.innerHeight - 10) {
      y = ev.clientY - rect.height - pad;
    }
    tt.style.left = Math.max(8, x) + "px";
    tt.style.top = Math.max(8, y) + "px";
  }

  function hideTooltip() {
    const tt = getTooltip();
    tt.style.display = "none";
  }

  function themeColors() {
    const dark = document.documentElement.getAttribute("data-theme") === "dark";
    return {
      text: dark ? "#cbd5e1" : "#475569",
      muted: dark ? "#94a3b8" : "#64748b",
      grid: dark ? "rgba(148,163,184,0.14)" : "rgba(15,23,42,0.08)",
      axis: dark ? "rgba(148,163,184,0.28)" : "rgba(15,23,42,0.18)",
      bg: dark ? "#111827" : "#ffffff"
    };
  }

  function compactNum(v, unit) {
    const abs = Math.abs(v);
    let str;
    if (abs >= 1e6) {
      str = (v / 1e6).toFixed(2).replace(".", ",") + " млн";
    } else if (abs >= 1e3) {
      str = (v / 1e3).toFixed(0) + " тыс.";
    } else if (Math.abs(v - Math.round(v)) < 1e-4) {
      str = String(Math.round(v));
    } else {
      str = v.toFixed(1).replace(".", ",");
    }
    return unit ? str + " " + unit : str;
  }

  function fullNum(v, suffix) {
    if (v == null || isNaN(v)) return "—";
    const rounded = Math.round(v * 100) / 100;
    const parts = rounded.toFixed(2).split(".");
    parts[0] = parts[0].replace(/\B(?=(\d{3})+(?!\d))/g, " ");
    const out = parts[1] === "00" ? parts[0] : parts[0] + "," + parts[1];
    return suffix ? out + " " + suffix : out;
  }

  function setupCanvas(canvas) {
    if (!canvas) return null;
    const dpr = window.devicePixelRatio || 1;
    const rect = canvas.parentElement.getBoundingClientRect();
    const w = Math.max(260, Math.floor(rect.width));
    const h = Math.max(180, Math.floor(rect.height));
    canvas.width = w * dpr;
    canvas.height = h * dpr;
    canvas.style.width = w + "px";
    canvas.style.height = h + "px";
    const ctx = canvas.getContext("2d");
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    return { ctx, w, h };
  }

  function renderComboChart(canvas, cfg) {
    const setup = setupCanvas(canvas);
    if (!setup) return;
    const { ctx, w, h } = setup;
    const tc = themeColors();
    const labels = cfg.labels || [];
    const series = cfg.series || [];
    const unit = cfg.unit || "";
    const rightUnit = cfg.rightUnit || "%";
    const stacked = !!cfg.stacked;

    if (!labels.length || !series.length) {
      ctx.fillStyle = tc.muted;
      ctx.font = "13px Segoe UI, sans-serif";
      ctx.fillText("Нет данных для отображения", 24, h / 2);
      return;
    }

    const hasRight = series.some((s) => s.yAxis === "right");
    const padTop = 36;
    const padBottom = labels.length > 8 ? 54 : 38;
    const padLeft = 68;
    const padRight = hasRight ? 54 : 20;
    const plotW = Math.max(60, w - padLeft - padRight);
    const plotH = Math.max(60, h - padTop - padBottom);

    const barSeries = series.filter((s) => (s.type || "bar") === "bar" && s.yAxis !== "right");
    const leftSeries = series.filter((s) => s.yAxis !== "right");

    let minLeft = 0;
    let maxLeft = 0;
    for (let i = 0; i < labels.length; i++) {
      if (stacked && barSeries.length > 0) {
        let posSum = 0;
        let negSum = 0;
        barSeries.forEach((s) => {
          const val = Number(s.data[i] || 0);
          if (val >= 0) posSum += val;
          else negSum += val;
        });
        maxLeft = Math.max(maxLeft, posSum);
        minLeft = Math.min(minLeft, negSum);
      }
      leftSeries.forEach((s) => {
        if (stacked && (s.type || "bar") === "bar") return;
        const val = Number(s.data[i] || 0);
        maxLeft = Math.max(maxLeft, val);
        minLeft = Math.min(minLeft, val);
      });
    }
    (cfg.refLines || []).forEach((rl) => {
      maxLeft = Math.max(maxLeft, rl.value);
    });
    if (maxLeft === minLeft) maxLeft = minLeft + 1;
    maxLeft *= 1.12;
    if (minLeft < 0) minLeft *= 1.12;

    let maxRight = cfg.maxRight || 100;
    series.filter((s) => s.yAxis === "right").forEach((s) => {
      s.data.forEach((v) => { maxRight = Math.max(maxRight, Number(v || 0)); });
    });

    const yLeft = (v) => padTop + plotH - ((v - minLeft) / (maxLeft - minLeft)) * plotH;
    const yRight = (v) => padTop + plotH - (v / maxRight) * plotH;
    const zeroY = yLeft(0);

    function draw(hoverIdx) {
      ctx.clearRect(0, 0, w, h);

      // Legend
      ctx.font = "600 12px Segoe UI, sans-serif";
      let lx = padLeft;
      series.forEach((s, idx) => {
        const col = s.color || PALETTE[idx % PALETTE.length];
        ctx.fillStyle = col;
        if (s.type === "line") {
          ctx.fillRect(lx, 13, 14, 3);
          ctx.beginPath();
          ctx.arc(lx + 7, 14.5, 3.5, 0, Math.PI * 2);
          ctx.fill();
        } else {
          ctx.fillRect(lx, 9, 11, 11);
        }
        ctx.fillStyle = tc.text;
        ctx.fillText(s.name, lx + 17, 19);
        lx += ctx.measureText(s.name).width + 34;
      });

      // Grid lines
      const steps = 5;
      ctx.font = "11px Segoe UI, sans-serif";
      for (let i = 0; i <= steps; i++) {
        const ratio = i / steps;
        const val = minLeft + (maxLeft - minLeft) * ratio;
        const yy = padTop + plotH - ratio * plotH;
        ctx.strokeStyle = tc.grid;
        ctx.lineWidth = 1;
        ctx.beginPath();
        ctx.moveTo(padLeft, yy);
        ctx.lineTo(padLeft + plotW, yy);
        ctx.stroke();

        ctx.fillStyle = tc.muted;
        ctx.textAlign = "right";
        ctx.fillText(compactNum(val, ""), padLeft - 8, yy + 4);

        if (hasRight) {
          const rVal = maxRight * ratio;
          ctx.textAlign = "left";
          ctx.fillText(Math.round(rVal) + rightUnit, padLeft + plotW + 8, yy + 4);
        }
      }

      // Hover band
      const stepW = plotW / labels.length;
      if (hoverIdx >= 0 && hoverIdx < labels.length) {
        ctx.fillStyle = "rgba(37, 99, 235, 0.06)";
        ctx.fillRect(padLeft + hoverIdx * stepW, padTop, stepW, plotH);
      }

      // Zero line
      if (minLeft < 0) {
        ctx.strokeStyle = tc.axis;
        ctx.beginPath();
        ctx.moveTo(padLeft, zeroY);
        ctx.lineTo(padLeft + plotW, zeroY);
        ctx.stroke();
      }

      // Bars
      if (barSeries.length > 0) {
        const groupW = stepW * 0.68;
        for (let i = 0; i < labels.length; i++) {
          const xStart = padLeft + i * stepW + (stepW - groupW) / 2;
          if (stacked) {
            let posAcc = 0;
            let negAcc = 0;
            barSeries.forEach((s) => {
              const sIdx = series.indexOf(s);
              const col = s.color || PALETTE[sIdx % PALETTE.length];
              const v = Number(s.data[i] || 0);
              ctx.fillStyle = col;
              if (v >= 0) {
                const y1 = yLeft(posAcc + v);
                const y0 = yLeft(posAcc);
                ctx.fillRect(xStart, y1, groupW, Math.max(1, y0 - y1));
                posAcc += v;
              } else {
                const y0 = yLeft(negAcc);
                const y1 = yLeft(negAcc + v);
                ctx.fillRect(xStart, y0, groupW, Math.max(1, y1 - y0));
                negAcc += v;
              }
            });
          } else {
            const subW = Math.max(3, (groupW - (barSeries.length - 1) * 3) / barSeries.length);
            barSeries.forEach((s, bIdx) => {
              const sIdx = series.indexOf(s);
              const col = s.color || PALETTE[sIdx % PALETTE.length];
              const v = Number(s.data[i] || 0);
              const bx = xStart + bIdx * (subW + 3);
              const vy = yLeft(v);
              ctx.fillStyle = col;
              if (v >= 0) {
                ctx.fillRect(bx, vy, subW, Math.max(1, zeroY - vy));
              } else {
                ctx.fillRect(bx, zeroY, subW, Math.max(1, vy - zeroY));
              }
            });
          }
        }
      }

      // Reference lines
      (cfg.refLines || []).forEach((rl) => {
        const ry = yLeft(rl.value);
        ctx.save();
        ctx.strokeStyle = rl.color || "#f59e0b";
        ctx.setLineDash([5, 4]);
        ctx.lineWidth = 1.5;
        ctx.beginPath();
        ctx.moveTo(padLeft, ry);
        ctx.lineTo(padLeft + plotW, ry);
        ctx.stroke();
        ctx.fillStyle = rl.color || "#f59e0b";
        ctx.font = "600 10.5px Segoe UI, sans-serif";
        ctx.textAlign = "right";
        ctx.fillText(rl.label || "", padLeft + plotW - 4, ry - 4);
        ctx.restore();
      });

      // Line series
      series.filter((s) => s.type === "line").forEach((s) => {
        const sIdx = series.indexOf(s);
        const col = s.color || PALETTE[sIdx % PALETTE.length];
        const mapY = s.yAxis === "right" ? yRight : yLeft;
        ctx.strokeStyle = col;
        ctx.lineWidth = 2.4;
        ctx.beginPath();
        for (let i = 0; i < labels.length; i++) {
          const px = padLeft + i * stepW + stepW / 2;
          const py = mapY(Number(s.data[i] || 0));
          if (i === 0) ctx.moveTo(px, py);
          else ctx.lineTo(px, py);
        }
        ctx.stroke();

        for (let i = 0; i < labels.length; i++) {
          const px = padLeft + i * stepW + stepW / 2;
          const py = mapY(Number(s.data[i] || 0));
          ctx.fillStyle = tc.bg;
          ctx.beginPath();
          ctx.arc(px, py, i === hoverIdx ? 5 : 3.5, 0, Math.PI * 2);
          ctx.fill();
          ctx.strokeStyle = col;
          ctx.lineWidth = 2;
          ctx.stroke();
        }
      });

      // X-axis labels
      ctx.fillStyle = tc.muted;
      ctx.font = "11px Segoe UI, sans-serif";
      const skip = labels.length > 16 ? Math.ceil(labels.length / 12) : 1;
      for (let i = 0; i < labels.length; i += skip) {
        const px = padLeft + i * stepW + stepW / 2;
        const lbl = String(labels[i]);
        ctx.save();
        if (labels.length > 8) {
          ctx.translate(px, padTop + plotH + 14);
          ctx.rotate(-0.36);
          ctx.textAlign = "right";
          ctx.fillText(lbl.length > 18 ? lbl.slice(0, 17) + "…" : lbl, 0, 0);
        } else {
          ctx.textAlign = "center";
          ctx.fillText(lbl.length > 22 ? lbl.slice(0, 21) + "…" : lbl, px, padTop + plotH + 18);
        }
        ctx.restore();
      }
    }

    draw(-1);

    canvas.onmousemove = function (ev) {
      const r = canvas.getBoundingClientRect();
      const mx = ev.clientX - r.left;
      const stepW = plotW / labels.length;
      const idx = Math.floor((mx - padLeft) / stepW);
      if (idx < 0 || idx >= labels.length) {
        draw(-1);
        hideTooltip();
        return;
      }
      draw(idx);
      let html = `<div style="font-weight:700;margin-bottom:5px;border-bottom:1px solid rgba(255,255,255,0.15);padding-bottom:4px">${labels[idx]}</div>`;
      series.forEach((s, sIdx) => {
        const col = s.color || PALETTE[sIdx % PALETTE.length];
        const v = Number(s.data[idx] || 0);
        const u = s.yAxis === "right" ? rightUnit : unit;
        html += `<div style="display:flex;justify-content:space-between;gap:14px;margin-top:3px">
          <span><span style="display:inline-block;width:8px;height:8px;border-radius:2px;background:${col};margin-right:6px"></span>${s.name}</span>
          <strong>${fullNum(v, u)}</strong>
        </div>`;
      });
      showTooltip(ev, html);
    };
    canvas.onmouseleave = function () {
      draw(-1);
      hideTooltip();
    };
  }

  function renderHorizBarChart(canvas, cfg) {
    const setup = setupCanvas(canvas);
    if (!setup) return;
    const { ctx, w, h } = setup;
    const tc = themeColors();
    const items = cfg.items || []; // [{label, value, sub}]
    const unit = cfg.unit || "";

    if (!items.length) return;

    const padLeft = cfg.padLeft || 175;
    const padRight = 84;
    const padTop = 14;
    const padBottom = 18;
    const plotW = Math.max(60, w - padLeft - padRight);
    const plotH = Math.max(60, h - padTop - padBottom);

    let minV = 0;
    let maxV = 0;
    items.forEach((it) => {
      minV = Math.min(minV, it.value);
      maxV = Math.max(maxV, it.value);
    });
    if (maxV === minV) maxV = minV + 1;
    if (maxV > 0) maxV *= 1.1;
    if (minV < 0) minV *= 1.1;

    const xOf = (v) => padLeft + ((v - minV) / (maxV - minV)) * plotW;
    const zeroX = xOf(0);
    const rowH = plotH / items.length;

    function draw(hoverIdx) {
      ctx.clearRect(0, 0, w, h);

      ctx.strokeStyle = tc.axis;
      ctx.beginPath();
      ctx.moveTo(zeroX, padTop);
      ctx.lineTo(zeroX, padTop + plotH);
      ctx.stroke();

      items.forEach((it, idx) => {
        const yCenter = padTop + idx * rowH + rowH / 2;
        const barH = Math.min(20, Math.max(8, rowH * 0.62));
        if (idx === hoverIdx) {
          ctx.fillStyle = "rgba(37,99,235,0.07)";
          ctx.fillRect(0, padTop + idx * rowH, w, rowH);
        }

        ctx.fillStyle = tc.text;
        ctx.font = "12px Segoe UI, sans-serif";
        ctx.textAlign = "right";
        const lbl = it.label.length > 25 ? it.label.slice(0, 24) + "…" : it.label;
        ctx.fillText(lbl, padLeft - 10, yCenter + 4);

        const vx = xOf(it.value);
        const col = it.color || (it.value >= 0 ? (cfg.color || "#2563eb") : "#ef4444");
        ctx.fillStyle = col;
        if (it.value >= 0) {
          ctx.fillRect(zeroX, yCenter - barH / 2, Math.max(2, vx - zeroX), barH);
          ctx.fillStyle = tc.muted;
          ctx.font = "600 11px Segoe UI, sans-serif";
          ctx.textAlign = "left";
          ctx.fillText(compactNum(it.value, ""), vx + 6, yCenter + 4);
        } else {
          ctx.fillRect(vx, yCenter - barH / 2, Math.max(2, zeroX - vx), barH);
          ctx.fillStyle = "#ef4444";
          ctx.font = "600 11px Segoe UI, sans-serif";
          ctx.textAlign = "left";
          ctx.fillText(compactNum(it.value, ""), zeroX + 6, yCenter + 4);
        }
      });
    }

    draw(-1);
    canvas.onmousemove = function (ev) {
      const r = canvas.getBoundingClientRect();
      const my = ev.clientY - r.top;
      const idx = Math.floor((my - padTop) / rowH);
      if (idx < 0 || idx >= items.length) {
        draw(-1);
        hideTooltip();
        return;
      }
      draw(idx);
      const it = items[idx];
      let html = `<div style="font-weight:700;margin-bottom:4px">${it.label}</div>
        <div>Значение: <strong>${fullNum(it.value, unit)}</strong></div>`;
      if (it.sub) {
        html += `<div style="color:#cbd5e1;margin-top:2px">${it.sub}</div>`;
      }
      showTooltip(ev, html);
    };
    canvas.onmouseleave = function () {
      draw(-1);
      hideTooltip();
    };
  }

  function renderDonutChart(canvas, cfg) {
    const setup = setupCanvas(canvas);
    if (!setup) return;
    const { ctx, w, h } = setup;
    const tc = themeColors();
    const items = (cfg.items || []).filter((x) => x.value > 0);
    const unit = cfg.unit || "";
    const total = items.reduce((a, b) => a + b.value, 0);
    if (!items.length || total <= 0) return;

    const cx = Math.min(w * 0.32, h * 0.5);
    const cy = h * 0.5;
    const outerR = Math.max(45, Math.min(cx - 16, h * 0.42));
    const innerR = outerR * 0.62;

    const slices = [];
    let angle = -Math.PI / 2;
    items.forEach((it, idx) => {
      const span = (it.value / total) * Math.PI * 2;
      slices.push({
        item: it,
        start: angle,
        end: angle + span,
        color: it.color || PALETTE[idx % PALETTE.length],
        share: (it.value / total) * 100
      });
      angle += span;
    });

    function draw(hoverIdx) {
      ctx.clearRect(0, 0, w, h);
      slices.forEach((sl, idx) => {
        ctx.beginPath();
        const rOut = idx === hoverIdx ? outerR + 5 : outerR;
        ctx.arc(cx, cy, rOut, sl.start, sl.end);
        ctx.arc(cx, cy, innerR, sl.end, sl.start, true);
        ctx.closePath();
        ctx.fillStyle = sl.color;
        ctx.fill();
      });

      // Center summary
      ctx.fillStyle = tc.muted;
      ctx.font = "600 11px Segoe UI, sans-serif";
      ctx.textAlign = "center";
      ctx.fillText(cfg.centerLabel || "ИТОГО", cx, cy - 6);
      ctx.fillStyle = tc.text;
      ctx.font = "700 14px Segoe UI, sans-serif";
      ctx.fillText(compactNum(total, ""), cx, cy + 14);

      // Legend on right
      const legX = cx + outerR + 24;
      const maxLeg = Math.min(slices.length, 8);
      const startY = Math.max(20, cy - (maxLeg * 24) / 2);
      ctx.textAlign = "left";
      for (let i = 0; i < maxLeg; i++) {
        const sl = slices[i];
        const yy = startY + i * 24;
        ctx.fillStyle = sl.color;
        ctx.fillRect(legX, yy - 9, 10, 10);
        ctx.fillStyle = tc.text;
        ctx.font = i === hoverIdx ? "700 12px Segoe UI, sans-serif" : "12px Segoe UI, sans-serif";
        const name = sl.item.label.length > 24 ? sl.item.label.slice(0, 23) + "…" : sl.item.label;
        ctx.fillText(`${name} (${sl.share.toFixed(1).replace(".", ",")}%)`, legX + 16, yy);
      }
    }

    draw(-1);
    canvas.onmousemove = function (ev) {
      const r = canvas.getBoundingClientRect();
      const dx = ev.clientX - r.left - cx;
      const dy = ev.clientY - r.top - cy;
      const dist = Math.sqrt(dx * dx + dy * dy);
      if (dist < innerR || dist > outerR + 8) {
        draw(-1);
        hideTooltip();
        return;
      }
      let ang = Math.atan2(dy, dx);
      if (ang < -Math.PI / 2) ang += Math.PI * 2;
      const idx = slices.findIndex((sl) => ang >= sl.start && ang <= sl.end);
      if (idx < 0) {
        draw(-1);
        hideTooltip();
        return;
      }
      draw(idx);
      const sl = slices[idx];
      showTooltip(
        ev,
        `<div style="font-weight:700;margin-bottom:4px">${sl.item.label}</div>
         <div>Сумма: <strong>${fullNum(sl.item.value, unit)}</strong></div>
         <div>Доля: <strong>${sl.share.toFixed(2).replace(".", ",")}%</strong></div>`
      );
    };
    canvas.onmouseleave = function () {
      draw(-1);
      hideTooltip();
    };
  }

  window.SalesCharts = {
    renderComboChart,
    renderHorizBarChart,
    renderDonutChart,
    compactNum,
    fullNum,
    PALETTE
  };
})();
