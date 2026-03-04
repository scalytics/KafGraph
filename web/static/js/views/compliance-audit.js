// Compliance Audit Trail View

import { api } from '../api.js';
import { timeline } from '../components/timeline.js';

export async function renderComplianceAudit(container) {
  container.innerHTML = '<div class="loading">Loading audit trail...</div>';

  try {
    const data = await api.complianceEvents({ limit: 100 });
    const events = data.events || [];

    let html = '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:var(--space-md)">';
    html += `<h3 style="margin:0">Compliance Audit Trail (${data.total || 0} events)</h3>`;
    html += '</div>';

    if (events.length === 0) {
      html += '<div class="card">No compliance events recorded yet.</div>';
    } else {
      const timelineItems = events.map(e => {
        const p = e.properties || e;
        return {
          date: shortDate(p.timestamp),
          text: `<strong>${eventLabel(p.eventType)}</strong> ${p.details || ''}<br/>` +
                `<span class="text-sm">${p.actor ? 'By: ' + p.actor : ''}</span>`,
        };
      });
      html += timeline(timelineItems);

      // Also show as a table for searchability
      html += '<div style="margin-top:var(--space-lg)">';
      html += '<h4>Event Log</h4>';
      html += '<table class="data-table"><thead><tr>';
      html += '<th>Time</th><th>Type</th><th>Actor</th><th>Details</th>';
      html += '</tr></thead><tbody>';
      for (const e of events) {
        const p = e.properties || e;
        html += '<tr>';
        html += `<td class="mono">${shortDate(p.timestamp)}</td>`;
        html += `<td>${eventBadge(p.eventType)}</td>`;
        html += `<td>${p.actor || '-'}</td>`;
        html += `<td>${p.details || ''}</td>`;
        html += '</tr>';
      }
      html += '</tbody></table>';
      html += '</div>';
    }

    container.innerHTML = html;
  } catch (err) {
    container.innerHTML = `<div class="card">Error: ${err.message}</div>`;
  }
}

function shortDate(iso) {
  if (!iso) return 'N/A';
  return iso.replace('T', ' ').substring(0, 19);
}

function eventLabel(type) {
  const labels = {
    inspection_created: 'Inspection Created',
    inspection_updated: 'Inspection Updated',
    inspection_signed_off: 'Inspection Signed Off',
    finding_opened: 'Finding Opened',
    finding_updated: 'Finding Updated',
    finding_resolved: 'Finding Resolved',
    remediation_created: 'Remediation Created',
    remediation_updated: 'Remediation Updated',
    dataflow_created: 'Data Flow Created',
    dataflow_validation: 'Data Flow Validation',
  };
  return labels[type] || type || 'Unknown';
}

function eventBadge(type) {
  const color = type?.includes('created') ? 'pass' :
                type?.includes('signed_off') ? 'pass' :
                type?.includes('opened') ? 'fail' :
                type?.includes('resolved') ? 'pass' : 'warning';
  return `<span class="status-badge ${color}">${eventLabel(type)}</span>`;
}
