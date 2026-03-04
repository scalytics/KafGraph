// Compliance Inspections View

import { api } from '../api.js';
import { dataTable } from '../components/table.js';

export async function renderComplianceInspections(container) {
  container.innerHTML = '<div class="loading">Loading inspections...</div>';

  try {
    const data = await api.inspections();
    const items = (data.items || []).map(n => {
      const p = n.properties || n;
      return { ...p, _id: n.id, findingCount: n.findingCount || p.findingCount || 0 };
    });

    let html = '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:var(--space-md)">';
    html += '<h3 style="margin:0">Compliance Inspections</h3>';
    html += '<button class="btn-scan" id="add-insp-btn">+ New Inspection</button>';
    html += '</div>';

    if (items.length === 0) {
      html += '<div class="card">No inspections yet. Create one to begin.</div>';
    } else {
      const columns = [
        { label: 'Title', key: 'title' },
        { label: 'Inspector', key: 'inspectorId' },
        { label: 'Status', key: 'status', render: row => statusBadge(row.status) },
        { label: 'Findings', key: 'findingCount' },
        { label: 'Created', key: 'createdAt', render: row => shortDate(row.createdAt) },
        { label: '', key: '_id', render: row => `<button class="btn-link view-insp" data-id="${row._id}">View</button>` },
      ];
      html += dataTable(columns, items);
    }

    container.innerHTML = html;

    // Wire buttons
    container.querySelector('#add-insp-btn')?.addEventListener('click', () => showInspectionForm(container));
    container.querySelectorAll('.view-insp').forEach(btn => {
      btn.addEventListener('click', () => showInspectionDetail(container, btn.dataset.id));
    });
  } catch (err) {
    container.innerHTML = `<div class="card">Error: ${err.message}</div>`;
  }
}

async function showInspectionDetail(container, id) {
  container.innerHTML = '<div class="loading">Loading inspection...</div>';

  try {
    const data = await api.inspectionDetail(id);
    const props = data.properties || data;
    const findings = data.findings || [];
    const scope = data.scope || [];

    let html = `<button class="btn-link" id="back-btn">&larr; Back to list</button>`;
    html += `<div class="card" style="margin-top:var(--space-md)">`;
    html += `<h3 style="margin-top:0">${props.title || 'Inspection'}</h3>`;
    html += `<div class="detail-grid">`;
    html += detailRow('Status', statusBadge(props.status));
    html += detailRow('Inspector', props.inspectorId || 'N/A');
    html += detailRow('Created', shortDate(props.createdAt));
    if (props.signedOffAt) html += detailRow('Signed Off', shortDate(props.signedOffAt));
    if (props.approverId) html += detailRow('Approver', props.approverId);
    html += `</div></div>`;

    // Scope
    if (scope.length > 0) {
      html += `<h4>Scope</h4><div class="card-grid">`;
      for (const s of scope) {
        const sp = s.properties || s;
        html += `<div class="card"><strong>${sp.name || s.id}</strong><br/><span class="text-sm">${s.label || ''}</span></div>`;
      }
      html += `</div>`;
    }

    // Findings
    html += `<div style="display:flex;justify-content:space-between;align-items:center;margin-top:var(--space-lg)">`;
    html += `<h4 style="margin:0">Findings (${findings.length})</h4>`;
    html += `<button class="btn-scan" id="add-finding-btn">+ Add Finding</button>`;
    html += `</div>`;

    if (findings.length > 0) {
      const fCols = [
        { label: 'Title', key: 'title' },
        { label: 'Severity', key: 'severity', render: row => `<span class="status-badge ${row.severity}">${row.severity || 'N/A'}</span>` },
        { label: 'Status', key: 'status', render: row => statusBadge(row.status) },
        { label: 'Remediations', key: 'remediationCount' },
      ];
      const fItems = findings.map(f => ({ ...(f.properties || f), remediationCount: f.remediationCount || 0 }));
      html += dataTable(fCols, fItems);
    }

    // Sign-off button
    if (props.status !== 'signed_off' && props.status !== 'closed') {
      html += `<div style="margin-top:var(--space-lg)"><button class="btn-scan" id="signoff-btn">Sign Off Inspection</button></div>`;
    }

    container.innerHTML = html;

    container.querySelector('#back-btn')?.addEventListener('click', () => renderComplianceInspections(container));
    container.querySelector('#add-finding-btn')?.addEventListener('click', () => showFindingForm(container, id));
    container.querySelector('#signoff-btn')?.addEventListener('click', async () => {
      const approver = prompt('Approver ID:');
      if (!approver) return;
      try {
        await api.inspectionSignOff(id, { approverId: approver });
        showInspectionDetail(container, id);
      } catch (err) {
        alert('Sign-off failed: ' + err.message);
      }
    });
  } catch (err) {
    container.innerHTML = `<div class="card">Error: ${err.message}</div>`;
  }
}

function showInspectionForm(container) {
  const form = document.createElement('div');
  form.className = 'card';
  form.style.marginBottom = 'var(--space-lg)';
  form.innerHTML = `
    <h3 style="margin-top:0">New Inspection</h3>
    <div class="compliance-form">
      <div class="field">
        <label>Title</label>
        <input type="text" id="insp-title" placeholder="e.g., Q1 2026 GDPR Review" />
      </div>
      <div class="field">
        <label>Inspector ID</label>
        <input type="text" id="insp-inspector" placeholder="e.g., dpo-maria" />
      </div>
      <div class="field">
        <label>Scan ID (optional)</label>
        <input type="text" id="insp-scan" placeholder="e.g., scan-1" />
      </div>
      <div class="field">
        <label>&nbsp;</label>
        <button class="btn-scan" id="insp-submit">Create</button>
      </div>
    </div>
  `;
  container.prepend(form);

  form.querySelector('#insp-submit').addEventListener('click', async () => {
    try {
      await api.inspectionCreate({
        properties: {
          title: form.querySelector('#insp-title').value,
          inspectorId: form.querySelector('#insp-inspector').value,
        },
        scanId: form.querySelector('#insp-scan').value,
        scopeNodeIds: [],
      });
      renderComplianceInspections(container);
    } catch (err) {
      alert('Error: ' + err.message);
    }
  });
}

function showFindingForm(container, inspectionId) {
  const form = document.createElement('div');
  form.className = 'card';
  form.style.marginBottom = 'var(--space-lg)';
  form.innerHTML = `
    <h3 style="margin-top:0">Add Finding</h3>
    <div class="compliance-form">
      <div class="field">
        <label>Title</label>
        <input type="text" id="finding-title" placeholder="Description of the finding" />
      </div>
      <div class="field">
        <label>Severity</label>
        <select id="finding-severity">
          <option value="low">Low</option>
          <option value="medium">Medium</option>
          <option value="high" selected>High</option>
          <option value="critical">Critical</option>
        </select>
      </div>
      <div class="field">
        <label>&nbsp;</label>
        <button class="btn-scan" id="finding-submit">Add Finding</button>
      </div>
    </div>
  `;
  container.prepend(form);

  form.querySelector('#finding-submit').addEventListener('click', async () => {
    try {
      await api.findingCreate(inspectionId, {
        properties: {
          title: form.querySelector('#finding-title').value,
          severity: form.querySelector('#finding-severity').value,
        },
      });
      showInspectionDetail(container, inspectionId);
    } catch (err) {
      alert('Error: ' + err.message);
    }
  });
}

function statusBadge(status) {
  const cls = {
    draft: 'na', in_progress: 'warning', review: 'warning',
    signed_off: 'pass', closed: 'pass',
    open: 'fail', remediated: 'pass', waived: 'na', accepted: 'pass',
  }[status] || '';
  return `<span class="status-badge ${cls}">${status || 'N/A'}</span>`;
}

function shortDate(iso) {
  if (!iso) return 'N/A';
  return iso.substring(0, 10);
}

function detailRow(label, value) {
  return `<div class="detail-row"><span class="detail-label">${label}</span><span>${value}</span></div>`;
}
