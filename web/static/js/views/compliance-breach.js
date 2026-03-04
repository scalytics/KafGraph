// Breach Register View

import { api } from '../api.js';
import { dataTable } from '../components/table.js';
import { timeline } from '../components/timeline.js';

export async function renderComplianceBreach(container) {
  container.innerHTML = '<div class="loading">Loading breach data...</div>';

  try {
    const data = await api.gdprBreaches();
    const items = (data.items || []).map(n => n.properties || n);

    let html = '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:var(--space-md)">';
    html += '<h3 style="margin:0">Data Breach Register (Art. 33-34)</h3>';
    html += '<button class="btn-scan" id="add-breach-btn">+ Report Breach</button>';
    html += '</div>';

    // Summary cards
    const total = items.length;
    const critical = items.filter(i => i.severity === 'critical' || i.severity === 'high').length;
    html += '<div class="card-grid" style="margin-bottom:var(--space-lg)">';
    html += `<div class="card metric-card"><div class="metric-label">Total Breaches</div><div class="metric-value">${total}</div></div>`;
    html += `<div class="card metric-card"><div class="metric-label">High/Critical</div><div class="metric-value" style="color:${critical > 0 ? 'var(--color-error)' : 'var(--color-success)'}">${critical}</div></div>`;
    html += `<div class="card metric-card"><div class="metric-label">72h Notification</div><div class="metric-value">Art. 33</div><div class="metric-sub">Supervisory authority deadline</div></div>`;
    html += '</div>';

    if (items.length > 0) {
      // Table
      const columns = [
        { label: 'Title', key: 'title' },
        { label: 'Severity', key: 'severity', render: v => `<span class="status-badge ${v || ''}">${v || 'N/A'}</span>` },
        { label: 'Discovered', key: 'discoveredAt', render: v => v ? new Date(v).toLocaleDateString() : 'N/A' },
        { label: 'Authority Notified', key: 'authorityNotifiedAt', render: v => v ? new Date(v).toLocaleDateString() : '<span class="status-badge fail">Not yet</span>' },
        { label: 'Status', key: 'status', render: v => `<span class="status-badge ${v || ''}">${v || 'N/A'}</span>` },
      ];
      html += dataTable(columns, items);

      // Timeline for each breach
      for (const breach of items) {
        const events = [];
        if (breach.discoveredAt) events.push({ date: new Date(breach.discoveredAt).toLocaleString(), text: 'Breach discovered' });
        if (breach.authorityNotifiedAt) events.push({ date: new Date(breach.authorityNotifiedAt).toLocaleString(), text: 'Supervisory authority notified (Art. 33)' });
        if (breach.subjectsNotifiedAt) events.push({ date: new Date(breach.subjectsNotifiedAt).toLocaleString(), text: 'Data subjects notified (Art. 34)' });
        if (events.length > 0) {
          html += `<div class="card" style="margin-top:var(--space-md)">`;
          html += `<h4 style="margin-top:0">${breach.title} - Timeline</h4>`;
          html += timeline(events);
          html += '</div>';
        }
      }
    } else {
      html += '<div class="card">No breaches recorded.</div>';
    }

    container.innerHTML = html;

    const addBtn = container.querySelector('#add-breach-btn');
    if (addBtn) {
      addBtn.addEventListener('click', async () => {
        const title = prompt('Breach title:');
        if (!title) return;
        const severity = prompt('Severity (low, medium, high, critical):') || 'medium';
        try {
          await api.gdprBreachCreate({
            title,
            severity,
            discoveredAt: new Date().toISOString(),
            status: 'investigating',
          });
          renderComplianceBreach(container);
        } catch (err) {
          alert('Error: ' + err.message);
        }
      });
    }
  } catch (err) {
    container.innerHTML = `<div class="card">Error: ${err.message}</div>`;
  }
}
