// RoPA (Record of Processing Activities) View

import { api } from '../api.js';
import { dataTable } from '../components/table.js';

export async function renderComplianceRopa(container) {
  container.innerHTML = '<div class="loading">Loading processing activities...</div>';

  try {
    const data = await api.gdprRopa();
    const items = (data.items || []).map(n => n.properties || n);

    let html = '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:var(--space-md)">';
    html += '<h3 style="margin:0">Record of Processing Activities (Art. 30)</h3>';
    html += '<button class="btn-scan" id="add-ropa-btn">+ Add Activity</button>';
    html += '</div>';

    if (items.length === 0) {
      html += '<div class="card">No processing activities registered yet.</div>';
    } else {
      const columns = [
        { label: 'Name', key: 'name' },
        { label: 'Purpose', key: 'purpose' },
        { label: 'Legal Basis', key: 'legalBasis' },
        { label: 'Retention', key: 'retentionPeriod', render: v => v || '<span class="status-badge fail">Missing</span>' },
        { label: 'Risk', key: 'riskLevel', render: v => `<span class="status-badge ${v || ''}">${v || 'N/A'}</span>` },
        { label: 'Status', key: 'status', render: v => `<span class="status-badge ${v || ''}">${v || 'N/A'}</span>` },
      ];
      html += dataTable(columns, items);
    }

    container.innerHTML = html;

    // Wire add button
    const addBtn = container.querySelector('#add-ropa-btn');
    if (addBtn) {
      addBtn.addEventListener('click', () => showRopaForm(container));
    }
  } catch (err) {
    container.innerHTML = `<div class="card">Error: ${err.message}</div>`;
  }
}

function showRopaForm(container) {
  const form = document.createElement('div');
  form.className = 'card';
  form.style.marginBottom = 'var(--space-lg)';
  form.innerHTML = `
    <h3 style="margin-top:0">New Processing Activity</h3>
    <div class="compliance-form">
      <div class="field">
        <label>Name</label>
        <input type="text" id="ropa-name" placeholder="e.g., Customer Analytics" />
      </div>
      <div class="field">
        <label>Purpose</label>
        <input type="text" id="ropa-purpose" placeholder="e.g., Product improvement" />
      </div>
      <div class="field">
        <label>Legal Basis</label>
        <select id="ropa-legal-basis">
          <option value="consent">Consent (Art. 6(1)(a))</option>
          <option value="contract">Contract (Art. 6(1)(b))</option>
          <option value="legitimate_interest">Legitimate Interest (Art. 6(1)(f))</option>
          <option value="legal_obligation">Legal Obligation (Art. 6(1)(c))</option>
        </select>
      </div>
      <div class="field">
        <label>Retention Period</label>
        <input type="text" id="ropa-retention" placeholder="e.g., 2 years" />
      </div>
      <div class="field">
        <label>Risk Level</label>
        <select id="ropa-risk">
          <option value="low">Low</option>
          <option value="medium">Medium</option>
          <option value="high">High</option>
        </select>
      </div>
      <div class="field">
        <label>&nbsp;</label>
        <button class="btn-scan" id="ropa-submit">Create</button>
      </div>
    </div>
  `;
  container.prepend(form);

  form.querySelector('#ropa-submit').addEventListener('click', async () => {
    try {
      await api.gdprRopaCreate({
        name: form.querySelector('#ropa-name').value,
        purpose: form.querySelector('#ropa-purpose').value,
        legalBasis: form.querySelector('#ropa-legal-basis').value,
        retentionPeriod: form.querySelector('#ropa-retention').value,
        riskLevel: form.querySelector('#ropa-risk').value,
        status: 'active',
      });
      renderComplianceRopa(container);
    } catch (err) {
      alert('Error: ' + err.message);
    }
  });
}
