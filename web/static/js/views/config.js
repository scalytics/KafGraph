// Configuration View — Structured admin tabs with source indicators

import { api } from '../api.js';
import { badge } from '../components/badge.js';

// Section order for the tabs (matches server-side grouping).
const SECTION_ORDER = [
  'Server', 'Storage', 'Kafka', 'S3 / MinIO',
  'Ingest', 'Reflection', 'Cluster',
];

// Icons for config value sources.
const SOURCE_ICONS = {
  default: { icon: '\u25CB', label: 'DEFAULT', cls: 'badge-neutral' },
  file:    { icon: '\u25A0', label: 'CFG.File', cls: 'badge-info' },
  env:     { icon: '\u25C6', label: 'ENV', cls: 'badge-purple' },
};

export async function renderConfig(container) {
  const [detailed, clusterInfo] = await Promise.all([
    api.configDetailed().catch(() => null),
    api.cluster().catch(() => null),
  ]);

  // Fall back to the basic config endpoint if detailed is unavailable.
  if (!detailed) {
    const cfg = await api.config().catch(() => null);
    container.innerHTML = cfg
      ? renderFallbackConfig(cfg, clusterInfo)
      : '<div class="card"><div class="empty-state">Configuration unavailable</div></div>';
    return;
  }

  // Build tab buttons from section order (filter to sections that exist).
  const sections = SECTION_ORDER.filter(s => detailed[s]);

  container.innerHTML = `
    <div class="card" style="margin-bottom: var(--space-md); padding: var(--space-sm) var(--space-md);">
      <div class="flex-gap" style="align-items:center; font-size:var(--font-size-xs); gap:var(--space-lg);">
        <span style="font-weight:600; color:var(--color-text-secondary);">Legend:</span>
        ${Object.values(SOURCE_ICONS).map(s =>
          `<span class="badge ${s.cls}" style="font-size:10px;">${s.icon} ${s.label}</span>`
        ).join('')}
      </div>
    </div>

    <div class="tabs" id="config-tabs">
      ${sections.map((s, i) =>
        `<div class="tab${i === 0 ? ' active' : ''}" data-tab="cfg-${slugify(s)}">${s}</div>`
      ).join('')}
      <div class="tab" data-tab="cfg-cluster-status">Cluster Status</div>
    </div>

    ${sections.map((s, i) => `
      <div class="tab-content${i === 0 ? ' active' : ''}" id="tab-cfg-${slugify(s)}">
        ${renderSettingsSection(s, detailed[s])}
      </div>
    `).join('')}

    <div class="tab-content" id="tab-cfg-cluster-status">
      ${renderClusterStatus(clusterInfo)}
    </div>
  `;

  // Tab switching
  container.querySelectorAll('.tab').forEach(tab => {
    tab.addEventListener('click', () => {
      container.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
      container.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
      tab.classList.add('active');
      const target = document.getElementById('tab-' + tab.dataset.tab);
      if (target) target.classList.add('active');
    });
  });
}

function renderSettingsSection(sectionName, settings) {
  if (!settings || settings.length === 0) {
    return '<div class="empty-state">No settings</div>';
  }

  const rows = settings.map(s => {
    const src = SOURCE_ICONS[s.source] || SOURCE_ICONS.default;
    const valueDisplay = formatValue(s.value);
    const defaultDisplay = formatValue(s.default);
    const isChanged = String(s.value) !== String(s.default);

    return `
      <tr>
        <td>
          <span class="badge ${src.cls}" style="font-size:9px; min-width:60px; text-align:center;" title="${src.label}: ${s.envVar}">
            ${src.icon} ${src.label}
          </span>
        </td>
        <td class="mono" style="font-size:var(--font-size-xs);">${s.key}</td>
        <td class="${isChanged ? 'config-changed' : ''}">${valueDisplay}</td>
        <td style="color:var(--color-text-muted); font-size:var(--font-size-xs);">${defaultDisplay}</td>
        <td class="mono" style="font-size:10px; color:var(--color-text-muted);">${s.envVar}</td>
      </tr>
    `;
  }).join('');

  return `
    <div class="card">
      <table class="data-table">
        <thead>
          <tr>
            <th style="width:80px;">Source</th>
            <th>Key</th>
            <th>Value</th>
            <th>Default</th>
            <th>Env Variable</th>
          </tr>
        </thead>
        <tbody>${rows}</tbody>
      </table>
    </div>
  `;
}

function renderClusterStatus(cluster) {
  if (!cluster) return '<div class="card"><div class="empty-state">Cluster info unavailable</div></div>';

  if (!cluster.enabled) {
    return `
      <div class="card">
        <div class="empty-state">
          Cluster mode is disabled. Running in single-node mode.
        </div>
      </div>
    `;
  }

  const members = cluster.members || [];
  const partitions = cluster.partitions || {};

  return `
    <div class="card" style="margin-bottom: var(--space-md);">
      <div class="section-header"><h3 class="section-title">Self</h3></div>
      ${fieldset('Local Node', [
        ['Name', cluster.self?.name],
        ['Address', cluster.self?.addr],
        ['RPC Port', cluster.self?.rpcPort],
        ['Bolt Port', cluster.self?.boltPort],
        ['HTTP Port', cluster.self?.httpPort],
      ])}
    </div>

    <div class="card" style="margin-bottom: var(--space-md);">
      <div class="section-header"><h3 class="section-title">Members (${members.length})</h3></div>
      <table class="data-table">
        <thead><tr><th>Name</th><th>Address</th><th>RPC</th><th>Bolt</th><th>HTTP</th></tr></thead>
        <tbody>
          ${members.map(m => `
            <tr>
              <td class="mono">${m.name}${m.name === cluster.self?.name ? ' ' + badge('SELF', 'info').trim() : ''}</td>
              <td class="mono">${m.addr}</td>
              <td>${m.rpcPort}</td>
              <td>${m.boltPort}</td>
              <td>${m.httpPort}</td>
            </tr>
          `).join('')}
        </tbody>
      </table>
    </div>

    <div class="card">
      <div class="section-header"><h3 class="section-title">Partition Map</h3></div>
      <div style="display:grid; grid-template-columns: repeat(auto-fill, minmax(80px, 1fr)); gap: 4px;">
        ${Object.entries(partitions).map(([p, owner]) => `
          <div class="badge badge-neutral" style="text-align:center;" title="Partition ${p} -> ${owner}">
            <strong>${p}</strong><br><span style="font-size:10px">${owner}</span>
          </div>
        `).join('')}
      </div>
    </div>
  `;
}

// Fallback rendering for when detailed config API is unavailable.
function renderFallbackConfig(cfg, clusterInfo) {
  return `
    <div class="tabs">
      <div class="tab active" data-tab="node-config">Node Configuration</div>
      <div class="tab" data-tab="cluster-config">Cluster</div>
    </div>
    <div class="tab-content active" id="tab-node-config">
      ${fieldset('Server', [
        ['Host', cfg.host],
        ['Port', cfg.port],
        ['Bolt Port', cfg.boltPort || cfg.bolt_port],
        ['Log Level', cfg.logLevel || cfg.log_level],
        ['Log Format', cfg.logFormat || cfg.log_format],
      ])}
      ${fieldset('Storage', [
        ['Engine', cfg.storageEngine || cfg.storage_engine],
        ['Data Dir', cfg.dataDir || cfg.data_dir],
      ])}
      ${fieldset('Kafka', [
        ['Brokers', cfg.kafka?.brokers],
        ['Group ID', cfg.kafka?.group_id],
        ['Topic Prefix', cfg.kafka?.topic_prefix],
      ])}
    </div>
    <div class="tab-content" id="tab-cluster-config">
      ${renderClusterStatus(clusterInfo)}
    </div>
  `;
}

function formatValue(value) {
  if (value === true) return badge('true', 'success');
  if (value === false) return badge('false', 'neutral');
  if (value == null || value === '') return '<span style="color:var(--color-text-muted)">--</span>';
  return `<span class="field-value">${value}</span>`;
}

function fieldset(legend, fields) {
  const rows = fields.map(([key, value]) => {
    const display = formatValue(value);
    return `
      <div class="field-row">
        <span class="field-key">${key}</span>
        ${display}
      </div>
    `;
  }).join('');

  return `
    <fieldset class="fieldset">
      <legend class="fieldset-legend">${legend}</legend>
      ${rows}
    </fieldset>
  `;
}

function slugify(s) {
  return s.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '');
}
