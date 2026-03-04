// Vertical Timeline Component

export function timeline(items) {
  // items: [{date, text, type?}]
  if (!items || items.length === 0) {
    return '<div class="card">No timeline events.</div>';
  }

  let html = '<div class="timeline">';
  for (const item of items) {
    html += `
      <div class="timeline-item">
        <div class="timeline-date">${item.date || ''}</div>
        <div class="timeline-text">${item.text || ''}</div>
      </div>
    `;
  }
  html += '</div>';
  return html;
}
