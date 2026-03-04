// Data Stats View — Charts and storage metrics

import { api } from '../api.js';
import { metricCard } from '../components/card.js';
import { dataTable } from '../components/table.js';

export async function renderDataStats(container) {
  const [stats, storageInfo] = await Promise.all([
    api.graphStats().catch(() => ({ nodes: { total: 0, byLabel: {} }, edges: { total: 0, byLabel: {} } })),
    api.storage().catch(() => null),
  ]);

  container.innerHTML = `
    <div class="chart-row">
      <div class="card">
        <div class="section-header"><h3 class="section-title">Nodes by Label</h3></div>
        <div id="nodes-chart" class="chart-container chart-container-sm"></div>
      </div>
      <div class="card">
        <div class="section-header"><h3 class="section-title">Edges by Type</h3></div>
        <div id="edges-chart" class="chart-container chart-container-sm"></div>
      </div>
    </div>

    ${storageInfo ? `
    <div class="card-grid">
      ${metricCard('Storage Engine', storageInfo.engine || 'badger')}
      ${metricCard('LSM Size', formatBytes(storageInfo.lsmSize))}
      ${metricCard('VLog Size', formatBytes(storageInfo.vlogSize))}
      ${metricCard('Data Dir', storageInfo.dataDir || '--')}
    </div>` : ''}

    <div id="audit-table"></div>
  `;

  renderBarChart('nodes-chart', stats.nodes?.byLabel ?? {}, '#3b82f6');
  renderBarChart('edges-chart', stats.edges?.byLabel ?? {}, '#059669');

  // Try to show AuditEvent stats if available
  try {
    const auditNodes = await api.graphExplore({ label: 'AuditEvent', limit: 20 });
    if (auditNodes.nodes && auditNodes.nodes.length > 0) {
      document.getElementById('audit-table').innerHTML = dataTable(
        [
          { label: 'ID', key: 'id', mono: true },
          { label: 'Action', render: r => r.properties?.action || '--' },
          { label: 'Created', render: r => r.createdAt ? new Date(r.createdAt).toLocaleString() : '--' },
        ],
        auditNodes.nodes,
        { title: 'Recent Audit Events' }
      );
    }
  } catch { /* ok */ }
}

function renderBarChart(elementId, data, color) {
  const el = document.getElementById(elementId);
  if (!el || typeof echarts === 'undefined') return;

  const labels = Object.keys(data);
  const values = Object.values(data);

  if (labels.length === 0) {
    el.innerHTML = '<div class="empty-state">No data</div>';
    return;
  }

  const chart = echarts.init(el);
  chart.setOption({
    tooltip: { trigger: 'axis' },
    grid: { left: 100, right: 20, top: 10, bottom: 30 },
    xAxis: { type: 'value' },
    yAxis: {
      type: 'category',
      data: labels,
      axisLabel: { fontSize: 11 },
    },
    series: [{
      type: 'bar',
      data: values,
      itemStyle: {
        color: color,
        borderRadius: [0, 3, 3, 0],
      },
      barMaxWidth: 24,
    }],
  });

  window.addEventListener('resize', () => chart.resize(), { once: true });
}

function formatBytes(bytes) {
  if (bytes == null || bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
}
