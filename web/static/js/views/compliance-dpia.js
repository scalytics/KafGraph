// DPIA (Data Protection Impact Assessment) View

import { api } from '../api.js';
import { dataTable } from '../components/table.js';
import { riskMatrix } from '../components/risk-matrix.js';

export async function renderComplianceDpia(container) {
  container.innerHTML = '<div class="loading">Loading DPIA data...</div>';

  try {
    const data = await api.gdprDpia();
    const items = (data.items || []).map(n => ({
      ...(n.properties || n),
      id: n.id,
    }));

    let html = '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:var(--space-md)">';
    html += '<h3 style="margin:0">Data Protection Impact Assessments (Art. 35)</h3>';
    html += '<button class="btn-scan" id="add-dpia-btn">+ New DPIA</button>';
    html += '</div>';

    if (items.length === 0) {
      html += '<div class="card">No DPIAs registered. High-risk processing requires a DPIA.</div>';
    } else {
      const columns = [
        { label: 'Title', key: 'title' },
        { label: 'Status', key: 'status', render: v => `<span class="status-badge ${v || ''}">${v || 'N/A'}</span>` },
        { label: 'Overall Risk', key: 'overallRisk', render: v => `<span class="status-badge ${v || ''}">${v || 'N/A'}</span>` },
        { label: 'Created', key: 'createdAt', render: v => v ? new Date(v).toLocaleDateString() : 'N/A' },
      ];
      html += dataTable(columns, items);

      // Show risk matrix placeholder (with sample data from graph if available)
      html += '<div class="card" style="margin-top:var(--space-lg)">';
      html += '<h4 style="margin-top:0">Risk Matrix</h4>';
      html += '<p style="font-size:var(--font-size-xs);color:var(--color-text-secondary)">5x5 Likelihood x Impact grid for identified risks</p>';
      // Use placeholder risks — real ones would come from DPIARisk nodes
      const sampleRisks = [
        { likelihood: 2, impact: 4 },
        { likelihood: 3, impact: 3 },
      ];
      html += riskMatrix(sampleRisks);
      html += '</div>';
    }

    container.innerHTML = html;

    const addBtn = container.querySelector('#add-dpia-btn');
    if (addBtn) {
      addBtn.addEventListener('click', async () => {
        const title = prompt('DPIA title:');
        if (!title) return;
        try {
          await api.gdprDpiaCreate({
            title,
            status: 'draft',
            overallRisk: 'tbd',
          });
          renderComplianceDpia(container);
        } catch (err) {
          alert('Error: ' + err.message);
        }
      });
    }
  } catch (err) {
    container.innerHTML = `<div class="card">Error: ${err.message}</div>`;
  }
}
