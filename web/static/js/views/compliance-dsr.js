// DSR (Data Subject Request) Tracker View

import { api } from '../api.js';
import { dataTable } from '../components/table.js';
import { slaBadge } from '../components/sla-indicator.js';

export async function renderComplianceDsr(container) {
  container.innerHTML = '<div class="loading">Loading DSR data...</div>';

  try {
    const [dsrData, slaData] = await Promise.all([
      api.gdprDsr(),
      api.gdprDsrSla().catch(() => ({ sla: [] })),
    ]);

    const items = (dsrData.items || []).map(n => n.properties || n);
    const slaItems = slaData.sla || [];

    let html = '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:var(--space-md)">';
    html += '<h3 style="margin:0">Data Subject Request Tracker (Art. 12-22)</h3>';
    html += '<button class="btn-scan" id="add-dsr-btn">+ New Request</button>';
    html += '</div>';

    // SLA summary
    const overdue = slaItems.filter(s => s.overdue).length;
    const active = slaItems.length;
    html += '<div class="card-grid" style="margin-bottom:var(--space-lg)">';
    html += `<div class="card metric-card"><div class="metric-label">Active DSRs</div><div class="metric-value">${active}</div></div>`;
    html += `<div class="card metric-card"><div class="metric-label">Overdue</div><div class="metric-value" style="color:${overdue > 0 ? 'var(--color-error)' : 'var(--color-success)'}">${overdue}</div></div>`;
    html += `<div class="card metric-card"><div class="metric-label">Deadline (30 days)</div><div class="metric-value">Art. 12(3)</div><div class="metric-sub">Maximum response time</div></div>`;
    html += '</div>';

    // SLA indicators
    if (slaItems.length > 0) {
      html += '<h4>SLA Status</h4>';
      html += '<div class="eval-list" style="margin-bottom:var(--space-lg)">';
      for (const s of slaItems) {
        html += `
          <div class="eval-item">
            <span class="eval-rule">${s.requestType || 'Unknown'}</span>
            <span style="font-family:var(--font-mono);font-size:var(--font-size-xs)">${s.nodeId}</span>
            ${slaBadge(s.daysLeft, s.overdue)}
          </div>
        `;
      }
      html += '</div>';
    }

    // All DSRs table
    if (items.length > 0) {
      const columns = [
        { label: 'Type', key: 'requestType' },
        { label: 'Status', key: 'status', render: v => `<span class="status-badge ${v || ''}">${v || 'N/A'}</span>` },
        { label: 'Received', key: 'receivedAt', render: v => v ? new Date(v).toLocaleDateString() : 'N/A' },
        { label: 'Deadline', key: 'deadline', render: v => v ? new Date(v).toLocaleDateString() : 'N/A' },
      ];
      html += '<h4>All Requests</h4>';
      html += dataTable(columns, items);
    }

    container.innerHTML = html;

    const addBtn = container.querySelector('#add-dsr-btn');
    if (addBtn) {
      addBtn.addEventListener('click', async () => {
        const type = prompt('Request type (access, erasure, rectification, portability, restriction, objection):');
        if (!type) return;
        const deadline = new Date();
        deadline.setDate(deadline.getDate() + 30);
        try {
          await api.gdprDsrCreate({
            requestType: type,
            status: 'pending',
            receivedAt: new Date().toISOString(),
            deadline: deadline.toISOString(),
          });
          renderComplianceDsr(container);
        } catch (err) {
          alert('Error: ' + err.message);
        }
      });
    }
  } catch (err) {
    container.innerHTML = `<div class="card">Error: ${err.message}</div>`;
  }
}
