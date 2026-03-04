// Metric Card Component

export function metricCard(label, value, sub) {
  return `
    <div class="card metric-card">
      <div class="metric-label">${label}</div>
      <div class="metric-value">${value}</div>
      ${sub ? `<div class="metric-sub">${sub}</div>` : ''}
    </div>
  `;
}
