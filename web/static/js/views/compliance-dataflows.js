// Data Flows View — CRUD, map, and validation

import { api } from '../api.js';
import { dataTable } from '../components/table.js';

export async function renderComplianceDataflows(container) {
  container.innerHTML = '<div class="loading">Loading data flows...</div>';

  try {
    const data = await api.dataFlows();
    const items = (data.items || []).map(n => {
      const p = n.properties || n;
      return {
        ...p,
        _id: n.id,
        fromNames: (n.fromNames || []).join(', ') || 'N/A',
        toNames: (n.toNames || []).join(', ') || 'N/A',
        categoryNames: (n.categoryNames || []).join(', ') || 'N/A',
      };
    });

    let html = '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:var(--space-md)">';
    html += '<h3 style="margin:0">Data Flows (GDPR Art. 30)</h3>';
    html += '<div>';
    html += '<button class="btn-scan" id="validate-flows-btn" style="margin-right:var(--space-sm)">Validate Flows</button>';
    html += '<button class="btn-scan" id="add-flow-btn">+ Add Flow</button>';
    html += '</div></div>';

    if (items.length === 0) {
      html += '<div class="card">No data flows defined yet.</div>';
    } else {
      const columns = [
        { label: 'Name', key: 'name' },
        { label: 'From', key: 'fromNames' },
        { label: 'To', key: 'toNames' },
        { label: 'Categories', key: 'categoryNames' },
        { label: 'Transfer', key: 'transferType', render: row => transferBadge(row.transferType) },
        { label: 'Safeguard', key: 'safeguard', render: row => row.safeguard || '-' },
      ];
      html += dataTable(columns, items);
    }

    // Validation results container
    html += '<div id="validation-results"></div>';

    container.innerHTML = html;

    container.querySelector('#add-flow-btn')?.addEventListener('click', () => showFlowForm(container));
    container.querySelector('#validate-flows-btn')?.addEventListener('click', () => runValidation(container));
  } catch (err) {
    container.innerHTML = `<div class="card">Error: ${err.message}</div>`;
  }
}

async function runValidation(container) {
  const resultsDiv = container.querySelector('#validation-results');
  if (!resultsDiv) return;

  resultsDiv.innerHTML = '<div class="loading">Running validation...</div>';

  try {
    const data = await api.dataFlowValidate({});

    let html = '<div class="card" style="margin-top:var(--space-lg)">';
    html += `<h3 style="margin-top:0">Validation Results</h3>`;
    html += `<div style="margin-bottom:var(--space-md);font-size:var(--font-size-sm)">`;
    html += `Total: ${data.total || 0} | `;
    html += `<span class="status-badge pass">Pass: ${data.pass || 0}</span> `;
    html += `<span class="status-badge fail">Fail: ${data.fail || 0}</span> `;
    html += `<span class="status-badge warning">Warnings: ${data.warnings || 0}</span>`;
    html += `</div>`;

    const results = data.results || [];
    if (results.length > 0) {
      for (const r of results) {
        html += `<div class="validation-item" style="border-left:3px solid ${colorFor(r.overall)};padding:var(--space-sm);margin-bottom:var(--space-sm);background:var(--color-bg-card)">`;
        html += `<div style="font-weight:600">${r.flowName || r.flowId}</div>`;
        html += `<div style="font-size:var(--font-size-xs)">Overall: ${statusBadge(r.overall)}</div>`;
        if (r.checks) {
          for (const c of r.checks) {
            html += `<div style="font-size:var(--font-size-xs);margin-left:var(--space-md)">`;
            html += `${statusIcon(c.status)} <strong>${c.ruleId}</strong>: ${c.details}`;
            html += `</div>`;
          }
        }
        html += `</div>`;
      }
    }
    html += '</div>';

    resultsDiv.innerHTML = html;
  } catch (err) {
    resultsDiv.innerHTML = `<div class="card">Validation error: ${err.message}</div>`;
  }
}

function showFlowForm(container) {
  const form = document.createElement('div');
  form.className = 'card';
  form.style.marginBottom = 'var(--space-lg)';
  form.innerHTML = `
    <h3 style="margin-top:0">New Data Flow</h3>
    <div class="compliance-form">
      <div class="field">
        <label>Name</label>
        <input type="text" id="flow-name" placeholder="e.g., Internal Analytics Pipeline" />
      </div>
      <div class="field">
        <label>Transfer Type</label>
        <select id="flow-transfer">
          <option value="internal">Internal</option>
          <option value="domestic">Domestic</option>
          <option value="international">International</option>
        </select>
      </div>
      <div class="field">
        <label>Safeguard (for international)</label>
        <input type="text" id="flow-safeguard" placeholder="e.g., SCC, Adequacy Decision" />
      </div>
      <div class="field">
        <label>Legal Basis</label>
        <input type="text" id="flow-legal-basis" placeholder="e.g., consent, contract" />
      </div>
      <div class="field">
        <label>&nbsp;</label>
        <button class="btn-scan" id="flow-submit">Create</button>
      </div>
    </div>
  `;
  container.prepend(form);

  form.querySelector('#flow-submit').addEventListener('click', async () => {
    const props = {
      name: form.querySelector('#flow-name').value,
      transferType: form.querySelector('#flow-transfer').value,
    };
    const safeguard = form.querySelector('#flow-safeguard').value;
    if (safeguard) props.safeguard = safeguard;
    const legalBasis = form.querySelector('#flow-legal-basis').value;
    if (legalBasis) props.legalBasis = legalBasis;

    try {
      await api.dataFlowCreate({ properties: props });
      renderComplianceDataflows(container);
    } catch (err) {
      alert('Error: ' + err.message);
    }
  });
}

function transferBadge(type) {
  const cls = type === 'international' ? 'fail' : (type === 'domestic' ? 'warning' : 'pass');
  return `<span class="status-badge ${cls}">${type || 'N/A'}</span>`;
}

function statusBadge(status) {
  const cls = { pass: 'pass', fail: 'fail', warning: 'warning', na: 'na' }[status] || '';
  return `<span class="status-badge ${cls}">${status || 'N/A'}</span>`;
}

function statusIcon(status) {
  return { pass: '&#10003;', fail: '&#10007;', warning: '&#9888;', na: '&#8212;' }[status] || '';
}

function colorFor(status) {
  return { pass: 'var(--color-pass)', fail: 'var(--color-fail)', warning: 'var(--color-warning)' }[status] || 'var(--color-border)';
}
