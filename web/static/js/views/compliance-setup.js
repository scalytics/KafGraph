// Compliance Setup View — Org info, data categories, legal bases, security measures

import { api } from '../api.js';
import { dataTable } from '../components/table.js';

export async function renderComplianceSetup(container) {
  container.innerHTML = '<div class="loading">Loading setup data...</div>';

  try {
    const [setup, categories, legalBases, measures] = await Promise.all([
      api.gdprSetup(),
      api.gdprDataCategories(),
      api.gdprLegalBases(),
      api.gdprSecurityMeasures(),
    ]);

    const orgProps = setup.properties || setup || {};
    const catItems = (categories.items || []).map(n => n.properties || n);
    const lbItems = (legalBases.items || []).map(n => n.properties || n);
    const smItems = (measures.items || []).map(n => n.properties || n);

    let html = '';

    // Org Setup Form
    html += '<div class="card" style="margin-bottom:var(--space-lg)">';
    html += '<h3 style="margin-top:0">Organisation Setup (Art. 37)</h3>';
    html += '<div class="compliance-form" id="org-form">';
    html += `<div class="field"><label>Organisation Name</label><input type="text" id="org-name" value="${orgProps.orgName || ''}" /></div>`;
    html += `<div class="field"><label>DPO Name</label><input type="text" id="org-dpo-name" value="${orgProps.dpoName || ''}" /></div>`;
    html += `<div class="field"><label>DPO Email</label><input type="email" id="org-dpo-email" value="${orgProps.dpoEmail || ''}" /></div>`;
    html += `<div class="field"><label>Supervisory Authority</label><input type="text" id="org-authority" value="${orgProps.supervisoryAuthority || ''}" /></div>`;
    html += `<div class="field"><label>Country</label><input type="text" id="org-country" value="${orgProps.country || ''}" /></div>`;
    html += `<div class="field"><label>&nbsp;</label><button class="btn-scan" id="save-org-btn">Save</button></div>`;
    html += '</div></div>';

    // Data Categories
    html += '<div class="card" style="margin-bottom:var(--space-lg)">';
    html += '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:var(--space-sm)">';
    html += '<h3 style="margin:0">Data Categories</h3>';
    html += '<button class="btn-scan" id="add-cat-btn" style="font-size:var(--font-size-xs)">+ Add</button>';
    html += '</div>';
    if (catItems.length > 0) {
      html += dataTable([
        { label: 'Name', key: 'name' },
        { label: 'Description', key: 'description' },
        { label: 'Special (Art. 9)', key: 'isSpecial', render: v => v ? '<span class="status-badge warning">Yes</span>' : 'No' },
      ], catItems);
    } else {
      html += '<p style="font-size:var(--font-size-sm);color:var(--color-text-secondary)">No data categories defined.</p>';
    }
    html += '</div>';

    // Legal Bases
    html += '<div class="card" style="margin-bottom:var(--space-lg)">';
    html += '<h3 style="margin-top:0">Legal Bases (Art. 6)</h3>';
    if (lbItems.length > 0) {
      html += dataTable([
        { label: 'Name', key: 'name' },
        { label: 'Article', key: 'article', mono: true },
      ], lbItems);
    } else {
      html += '<p style="font-size:var(--font-size-sm);color:var(--color-text-secondary)">No legal bases defined. Seed demo data to populate.</p>';
    }
    html += '</div>';

    // Security Measures (TOMs)
    html += '<div class="card">';
    html += '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:var(--space-sm)">';
    html += '<h3 style="margin:0">Security Measures (Art. 32 TOMs)</h3>';
    html += '<button class="btn-scan" id="add-sm-btn" style="font-size:var(--font-size-xs)">+ Add</button>';
    html += '</div>';
    if (smItems.length > 0) {
      html += dataTable([
        { label: 'Name', key: 'name' },
        { label: 'Type', key: 'type', render: v => `<span class="status-badge ${v || ''}">${v || 'N/A'}</span>` },
        { label: 'Status', key: 'status', render: v => `<span class="status-badge ${v || ''}">${v || 'N/A'}</span>` },
      ], smItems);
    } else {
      html += '<p style="font-size:var(--font-size-sm);color:var(--color-text-secondary)">No security measures defined.</p>';
    }
    html += '</div>';

    container.innerHTML = html;

    // Wire save button
    container.querySelector('#save-org-btn')?.addEventListener('click', async () => {
      try {
        await api.gdprSetupUpdate({
          orgName: container.querySelector('#org-name').value,
          dpoName: container.querySelector('#org-dpo-name').value,
          dpoEmail: container.querySelector('#org-dpo-email').value,
          supervisoryAuthority: container.querySelector('#org-authority').value,
          country: container.querySelector('#org-country').value,
        });
        alert('Organisation setup saved.');
      } catch (err) {
        alert('Error: ' + err.message);
      }
    });

  } catch (err) {
    container.innerHTML = `<div class="card">Error: ${err.message}</div>`;
  }
}
