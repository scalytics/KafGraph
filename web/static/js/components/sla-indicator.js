// SLA Indicator Component

export function slaBadge(daysLeft, overdue) {
  if (overdue) {
    return `<span class="sla-badge overdue">${Math.abs(daysLeft)}d overdue</span>`;
  }
  if (daysLeft <= 7) {
    return `<span class="sla-badge warning">${daysLeft}d left</span>`;
  }
  return `<span class="sla-badge ok">${daysLeft}d left</span>`;
}
