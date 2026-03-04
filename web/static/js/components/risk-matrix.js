// 5x5 Risk Matrix Component for DPIA

export function riskMatrix(risks) {
  // risks: [{likelihood, impact}]
  const riskMap = {};
  for (const r of risks) {
    const key = `${r.likelihood}-${r.impact}`;
    riskMap[key] = (riskMap[key] || 0) + 1;
  }

  function cellClass(l, i) {
    const score = l * i;
    if (score >= 15) return 'critical';
    if (score >= 9)  return 'high';
    if (score >= 4)  return 'medium';
    return 'low';
  }

  let html = '<div class="risk-matrix">';

  // Y-axis labels + cells (top row = likelihood 5)
  for (let l = 5; l >= 1; l--) {
    html += `<div class="label">${l}</div>`;
    for (let i = 1; i <= 5; i++) {
      const key = `${l}-${i}`;
      const count = riskMap[key] || 0;
      html += `<div class="cell ${cellClass(l, i)}">${count ? '<span class="dot"></span>' : ''}</div>`;
    }
  }

  // X-axis labels
  html += '<div class="label"></div>';
  for (let i = 1; i <= 5; i++) {
    html += `<div class="label">${i}</div>`;
  }

  html += '</div>';
  html += '<div style="font-size:10px;color:var(--color-text-muted);margin-top:4px;display:flex;justify-content:space-between;max-width:320px">';
  html += '<span>Impact &rarr;</span><span>&uarr; Likelihood</span>';
  html += '</div>';

  return html;
}
