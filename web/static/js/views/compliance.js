// KafGraph Compliance Dashboard — Main view with tab sub-routing

import { api } from '../api.js';
import { scoreRing, progressBar } from '../components/progress-bar.js';
import { renderComplianceRopa } from './compliance-ropa.js';
import { renderComplianceDsr } from './compliance-dsr.js';
import { renderComplianceBreach } from './compliance-breach.js';
import { renderComplianceDpia } from './compliance-dpia.js';
import { renderComplianceSetup } from './compliance-setup.js';
import { renderComplianceInspections } from './compliance-inspections.js';
import { renderComplianceDataflows } from './compliance-dataflows.js';
import { renderComplianceAudit } from './compliance-audit.js';

const TABS = [
  { id: 'overview',    label: 'Overview' },
  { id: 'ropa',        label: 'RoPA' },
  { id: 'dsr',         label: 'DSR Tracker' },
  { id: 'breach',      label: 'Breach Register' },
  { id: 'dpia',        label: 'DPIA' },
  { id: 'dataflows',   label: 'Data Flows' },
  { id: 'inspections', label: 'Inspections' },
  { id: 'audit',       label: 'Audit Trail' },
  { id: 'setup',       label: 'Setup' },
];

let currentTab = 'overview';

export async function renderCompliance(container) {
  container.innerHTML = '';

  // Tab bar
  const tabBar = document.createElement('div');
  tabBar.className = 'compliance-tabs';
  for (const tab of TABS) {
    const btn = document.createElement('button');
    btn.className = 'compliance-tab' + (tab.id === currentTab ? ' active' : '');
    btn.textContent = tab.label;
    btn.addEventListener('click', () => {
      currentTab = tab.id;
      renderCompliance(container);
    });
    tabBar.appendChild(btn);
  }
  container.appendChild(tabBar);

  const content = document.createElement('div');
  container.appendChild(content);

  switch (currentTab) {
    case 'overview': await renderOverview(content); break;
    case 'ropa':     await renderComplianceRopa(content); break;
    case 'dsr':      await renderComplianceDsr(content); break;
    case 'breach':   await renderComplianceBreach(content); break;
    case 'dpia':        await renderComplianceDpia(content); break;
    case 'dataflows':   await renderComplianceDataflows(content); break;
    case 'inspections': await renderComplianceInspections(content); break;
    case 'audit':       await renderComplianceAudit(content); break;
    case 'setup':       await renderComplianceSetup(content); break;
  }
}

async function renderOverview(container) {
  container.innerHTML = '<div class="loading">Loading compliance data...</div>';

  try {
    const [dashboard, scoreData] = await Promise.all([
      api.complianceDashboard(),
      api.complianceScore().catch(() => ({ scores: [] })),
    ]);

    let html = '';

    // Score cards row
    html += '<div class="card-grid">';
    if (scoreData.scores && scoreData.scores.length > 0) {
      for (const s of scoreData.scores) {
        const score = typeof s.score === 'number' ? s.score : 0;
        const detail = `${s.passCount || 0} pass / ${s.failCount || 0} fail`;
        html += scoreRing(score, (s.framework || '').toUpperCase(), detail);
      }
    } else {
      html += scoreRing(0, 'No Scans', 'Run a scan to see scores');
    }
    html += `
      <div class="card" style="display:flex;align-items:center;justify-content:center">
        <div style="text-align:center">
          <div style="font-size:var(--font-size-2xl);font-weight:700">${dashboard.totalRules || 0}</div>
          <div style="font-size:var(--font-size-xs);color:var(--color-text-secondary)">RULES REGISTERED</div>
        </div>
      </div>
    `;
    html += '</div>';

    // Scan button
    html += '<div style="margin-bottom:var(--space-lg)">';
    html += '<button class="btn-scan" id="run-scan-btn">Run GDPR Scan</button>';
    html += '</div>';

    // Latest scan results
    if (dashboard.latestScan) {
      const scan = dashboard.latestScan;
      const props = scan.properties || scan;
      html += '<div class="card" style="margin-bottom:var(--space-lg)">';
      html += `<h3 style="margin:0 0 var(--space-sm)">Latest Scan</h3>`;
      html += `<div style="font-size:var(--font-size-sm);color:var(--color-text-secondary)">`;
      html += `ID: ${props.scanId || 'N/A'} | `;
      html += `Completed: ${props.completedAt || 'N/A'} | `;
      html += `Score: ${props.score || 0}%`;
      html += '</div>';
      html += `<div style="margin-top:var(--space-sm)">${progressBar(props.score || 0)}</div>`;
      html += '</div>';
    }

    // Module scores
    if (dashboard.moduleScores && Object.keys(dashboard.moduleScores).length > 0) {
      html += '<h3>Module Breakdown</h3>';
      html += '<div class="module-grid">';
      for (const [mod, counts] of Object.entries(dashboard.moduleScores)) {
        const pass = counts.pass || 0;
        const fail = counts.fail || 0;
        const total = pass + fail + (counts.warning || 0);
        const pct = total > 0 ? Math.round((pass / total) * 100) : 0;
        html += `
          <div class="module-card">
            <div class="module-title">${mod.toUpperCase()}</div>
            <div class="module-desc">${pass} pass, ${fail} fail</div>
            ${progressBar(pct)}
          </div>
        `;
      }
      html += '</div>';
    }

    // Frameworks
    if (dashboard.frameworks && dashboard.frameworks.length > 0) {
      html += '<h3>Frameworks</h3>';
      html += '<div class="card-grid">';
      for (const fw of dashboard.frameworks) {
        const props = fw.properties || fw;
        html += `
          <div class="card">
            <div style="font-weight:600">${(props.name || '').toUpperCase()}</div>
            <div style="font-size:var(--font-size-xs);color:var(--color-text-secondary)">
              Version: ${props.version || 'N/A'} | Status: ${props.status || 'N/A'}
            </div>
          </div>
        `;
      }
      html += '</div>';
    }

    container.innerHTML = html;

    // Wire scan button
    const scanBtn = container.querySelector('#run-scan-btn');
    if (scanBtn) {
      scanBtn.addEventListener('click', async () => {
        scanBtn.disabled = true;
        scanBtn.textContent = 'Scanning...';
        try {
          const result = await api.complianceScan({ framework: 'gdpr' });
          alert(`Scan complete: Score ${result.score}% (${result.passCount} pass, ${result.failCount} fail)`);
          renderOverview(container);
        } catch (err) {
          alert('Scan failed: ' + err.message);
          scanBtn.disabled = false;
          scanBtn.textContent = 'Run GDPR Scan';
        }
      });
    }
  } catch (err) {
    container.innerHTML = `<div class="card">Error: ${err.message}</div>`;
  }
}
