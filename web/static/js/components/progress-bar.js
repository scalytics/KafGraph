// Compliance Progress Bar Component

export function progressBar(value, max = 100) {
  const pct = max > 0 ? Math.min(100, (value / max) * 100) : 0;
  const cls = pct >= 80 ? 'green' : pct >= 50 ? 'yellow' : 'red';
  return `<div class="progress-bar"><div class="fill ${cls}" style="width:${pct}%"></div></div>`;
}

export function scoreRing(score, label, detail) {
  const r = 34;
  const c = 2 * Math.PI * r;
  const pct = Math.min(100, Math.max(0, score));
  const offset = c - (pct / 100) * c;
  const color = pct >= 80 ? 'var(--color-success)' : pct >= 50 ? 'var(--color-warning)' : 'var(--color-error)';

  return `
    <div class="compliance-score-card card">
      <div class="score-ring">
        <svg viewBox="0 0 80 80">
          <circle class="ring-bg" cx="40" cy="40" r="${r}" />
          <circle class="ring-fg" cx="40" cy="40" r="${r}"
            stroke="${color}"
            stroke-dasharray="${c}"
            stroke-dashoffset="${offset}" />
        </svg>
        <div class="score-text" style="color:${color}">${Math.round(pct)}%</div>
      </div>
      <div class="score-meta">
        <div class="score-label">${label}</div>
        <div class="score-detail">${detail}</div>
      </div>
    </div>
  `;
}
