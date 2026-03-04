// Dashboard View — Overview cards + activity feed

import { api } from '../api.js';
import { metricCard } from '../components/card.js';
import { badge } from '../components/badge.js';

let refreshTimer = null;

export async function renderDashboard(container) {
  if (refreshTimer) clearInterval(refreshTimer);

  const [stats, info, reflectSummary, activity] = await Promise.all([
    api.graphStats().catch(() => ({ nodes: { total: 0, byLabel: {} }, edges: { total: 0, byLabel: {} } })),
    api.info().catch(() => null),
    api.reflectSummary().catch(() => null),
    api.activity({ hours: 24, limit: 20 }).catch(() => ({ events: [] })),
  ]);

  const activeCycles = reflectSummary?.byStatus?.RUNNING ?? 0;
  const feedbackPending = reflectSummary?.feedbackPipeline?.NEEDS_FEEDBACK ?? 0;

  container.innerHTML = `
    <div class="card-grid">
      ${metricCard('Total Nodes', (stats.nodes?.total ?? 0).toLocaleString(),
        `${Object.keys(stats.nodes?.byLabel ?? {}).length} labels`)}
      ${metricCard('Total Edges', (stats.edges?.total ?? 0).toLocaleString(),
        `${Object.keys(stats.edges?.byLabel ?? {}).length} types`)}
      ${metricCard('Active Cycles', activeCycles, reflectSummary ? `${reflectSummary.totalCycles ?? 0} total` : 'N/A')}
      ${metricCard('Feedback Pending', feedbackPending, reflectSummary ? `${reflectSummary.feedbackPipeline?.REQUESTED ?? 0} requested` : 'N/A')}
    </div>
    <div class="grid-2">
      <div>
        <div class="card">
          <div class="section-header">
            <h3 class="section-title">Recent Activity</h3>
          </div>
          <div id="activity-list">${renderActivityList(activity?.events)}</div>
        </div>
      </div>
      <div>
        <div class="card" style="margin-bottom: var(--space-md);">
          <div class="section-header">
            <h3 class="section-title">Node Distribution</h3>
          </div>
          <div id="node-dist-chart" class="chart-container chart-container-sm"></div>
        </div>
        <div class="card">
          <div class="section-header">
            <h3 class="section-title">Service Health</h3>
          </div>
          ${renderServiceHealth(info)}
        </div>
      </div>
    </div>
  `;

  renderNodeDistChart(stats);

  // Auto-refresh every 30s
  refreshTimer = setInterval(() => {
    renderDashboard(container);
  }, 30000);
}

function renderActivityList(events) {
  if (!events || events.length === 0) {
    return '<div class="empty-state">No recent activity</div>';
  }

  return events.map(ev => `
    <div class="field-row">
      <span>
        ${badge(ev.label || ev.type || 'Event')}
        <span class="mono" style="font-size: var(--font-size-xs); color: var(--color-text-secondary);">${ev.id || ''}</span>
      </span>
      <span style="font-size: var(--font-size-xs); color: var(--color-text-muted);">
        ${ev.createdAt ? new Date(ev.createdAt).toLocaleTimeString() : ''}
      </span>
    </div>
  `).join('');
}

function renderServiceHealth(info) {
  if (!info) return '<div class="empty-state">Service info unavailable</div>';

  return `
    <div class="field-row">
      <span class="field-key">Version</span>
      <span class="field-value">${info.version}</span>
    </div>
    <div class="field-row">
      <span class="field-key">Commit</span>
      <span class="field-value">${info.commit}</span>
    </div>
    <div class="field-row">
      <span class="field-key">Uptime</span>
      <span class="field-value">${info.uptime}</span>
    </div>
    <div class="field-row">
      <span class="field-key">Go Version</span>
      <span class="field-value">${info.goVersion}</span>
    </div>
    <div class="field-row">
      <span class="field-key">Storage</span>
      <span class="field-value">${info.storageEngine}</span>
    </div>
    <div class="field-row">
      <span class="field-key">OS / Arch</span>
      <span class="field-value">${info.os}/${info.arch}</span>
    </div>
  `;
}

function renderNodeDistChart(stats) {
  const el = document.getElementById('node-dist-chart');
  if (!el || typeof echarts === 'undefined') return;

  const byLabel = stats.nodes?.byLabel ?? {};
  const labels = Object.keys(byLabel);
  const values = Object.values(byLabel);

  if (labels.length === 0) {
    el.innerHTML = '<div class="empty-state">No nodes yet</div>';
    return;
  }

  const chart = echarts.init(el);
  chart.setOption({
    tooltip: { trigger: 'axis' },
    grid: { left: 80, right: 20, top: 10, bottom: 30 },
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
        color: '#3b82f6',
        borderRadius: [0, 3, 3, 0],
      },
      barMaxWidth: 24,
    }],
  });

  window.addEventListener('resize', () => chart.resize(), { once: true });
}
