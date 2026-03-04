// Sortable Data Table Component

export function dataTable(columns, rows, opts = {}) {
  if (!rows || rows.length === 0) {
    return `<div class="empty-state">No data available</div>`;
  }

  const headerCells = columns.map(col => {
    const cls = col.mono ? ' class="mono"' : '';
    return `<th${cls}>${col.label}</th>`;
  }).join('');

  const bodyRows = rows.map(row => {
    const cells = columns.map(col => {
      const val = col.render ? col.render(row) : (row[col.key] ?? '');
      const cls = col.mono ? ' class="mono"' : '';
      return `<td${cls}>${val}</td>`;
    }).join('');
    return `<tr>${cells}</tr>`;
  }).join('');

  return `
    <div class="card">
      ${opts.title ? `<div class="section-header"><h3 class="section-title">${opts.title}</h3></div>` : ''}
      <table class="data-table">
        <thead><tr>${headerCells}</tr></thead>
        <tbody>${bodyRows}</tbody>
      </table>
    </div>
  `;
}
