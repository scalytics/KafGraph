// Reflection View — Cycle table, feedback pipeline, score charts, trigger controls

import { api } from '../api.js';
import { metricCard } from '../components/card.js';
import { badge } from '../components/badge.js';

export async function renderReflection(container) {
  const [summary, cyclesResult] = await Promise.all([
    api.reflectSummary().catch(() => null),
    api.reflectCycles({ limit: 20, offset: 0 }).catch(() => ({ cycles: [] })),
  ]);

  if (!summary) {
    container.innerHTML = '<div class="card"><div class="empty-state">Reflection data unavailable</div></div>';
    return;
  }

  const avgScores = summary.averageScores || {};
  const pipeline = summary.feedbackPipeline || {};
  const lastRun = summary.lastRun;
  const lastRunByAgent = summary.lastRunByAgent || {};

  container.innerHTML = `
    <div class="card" style="margin-bottom: var(--space-lg);">
      <div class="section-header">
        <h3 class="section-title">Trigger Reflection</h3>
        <div class="flex-gap" style="align-items:center;">
          <span class="last-run-label">${lastRun ? 'Last run: ' + formatTimestamp(lastRun) : 'No cycles run yet'}</span>
        </div>
      </div>
      <div style="display:flex; gap:var(--space-md); flex-wrap:wrap; align-items:flex-end;">
        <div style="flex:1; min-width:200px;">
          <label style="display:block; font-size:var(--font-size-xs); color:var(--color-text-secondary); margin-bottom:4px;">Agent</label>
          <select class="form-select" id="trigger-agent" style="width:100%;">
            <option value="">All agents</option>
            ${Object.keys(lastRunByAgent).map(a =>
              `<option value="${a}">${a}</option>`
            ).join('')}
          </select>
        </div>
        <div style="min-width:120px;">
          <label style="display:block; font-size:var(--font-size-xs); color:var(--color-text-secondary); margin-bottom:4px;">Window (hours)</label>
          <select class="form-select" id="trigger-window" style="width:100%;">
            <option value="6">6h</option>
            <option value="12">12h</option>
            <option value="24" selected>24h</option>
            <option value="48">48h</option>
            <option value="168">7d</option>
          </select>
        </div>
        <button class="btn btn-primary" id="trigger-btn">Run Reflection</button>
      </div>
      <div id="trigger-result" style="margin-top:var(--space-md);"></div>
      ${Object.keys(lastRunByAgent).length > 0 ? `
        <details style="margin-top:var(--space-md);">
          <summary style="cursor:pointer; font-size:var(--font-size-sm); color:var(--color-text-secondary);">Last run per agent</summary>
          <div style="margin-top:var(--space-sm);">
            ${Object.entries(lastRunByAgent).map(([agent, ts]) => `
              <div class="field-row">
                <span class="field-key">${agent}</span>
                <span class="field-value">${formatTimestamp(ts)}</span>
              </div>
            `).join('')}
          </div>
        </details>
      ` : ''}
    </div>

    <div class="card-grid">
      ${metricCard('Total Cycles', summary.totalCycles ?? 0)}
      ${metricCard('Total Signals', summary.totalSignals ?? 0)}
      ${metricCard('Avg Impact', (avgScores.impact ?? 0).toFixed(2))}
      ${metricCard('Avg Relevance', (avgScores.relevance ?? 0).toFixed(2))}
      ${metricCard('Avg Value', (avgScores.valueContribution ?? 0).toFixed(2))}
    </div>

    <div class="grid-2">
      <div>
        <div class="card" style="margin-bottom: var(--space-md);">
          <div class="section-header">
            <h3 class="section-title">Feedback Pipeline</h3>
          </div>
          <div id="pipeline-chart" class="chart-container chart-container-sm"></div>
        </div>
      </div>
      <div>
        <div class="card" style="margin-bottom: var(--space-md);">
          <div class="section-header">
            <h3 class="section-title">Cycles by Type</h3>
          </div>
          <div id="type-chart" class="chart-container chart-container-sm"></div>
        </div>
      </div>
    </div>

    <div class="card">
      <div class="section-header">
        <h3 class="section-title">Recent Cycles</h3>
        <div class="flex-gap">
          <select class="form-select" id="cycle-type-filter">
            <option value="">All types</option>
            <option value="daily">Daily</option>
            <option value="weekly">Weekly</option>
            <option value="monthly">Monthly</option>
          </select>
          <select class="form-select" id="cycle-status-filter">
            <option value="">All statuses</option>
            <option value="COMPLETED">Completed</option>
            <option value="RUNNING">Running</option>
          </select>
        </div>
      </div>
      <div id="cycles-table">${renderCyclesTable(cyclesResult?.cycles || [])}</div>
      <div id="cycles-pagination" class="pagination"></div>
    </div>

    <div class="chart-row">
      <div class="card">
        <div class="section-header"><h3 class="section-title">Score Distribution</h3></div>
        <div id="score-chart" class="chart-container chart-container-sm"></div>
      </div>
    </div>
  `;

  renderPipelineChart(pipeline);
  renderTypeChart(summary.byType || {});
  renderScoreChart(avgScores);

  // Trigger button handler
  const triggerBtn = document.getElementById('trigger-btn');
  const triggerResult = document.getElementById('trigger-result');
  triggerBtn.addEventListener('click', async () => {
    triggerBtn.disabled = true;
    triggerBtn.textContent = 'Running...';
    triggerResult.innerHTML = '';
    try {
      const agentId = document.getElementById('trigger-agent').value;
      const windowHours = parseInt(document.getElementById('trigger-window').value, 10);
      const result = await api.reflectTrigger({ agentId, windowHours });
      const items = (result.results || []).map(r => {
        if (r.error) {
          return `<div style="color:var(--color-error);">${r.agentId}: ${r.error}</div>`;
        }
        return `<div>${badge(r.agentId, 'info')} ${r.signals} signals -- ${r.summary?.slice(0, 100) || ''}</div>`;
      }).join('');
      triggerResult.innerHTML = `
        <div class="card" style="background:var(--color-content-bg); padding:var(--space-md);">
          <strong>${result.triggered} cycle(s) triggered</strong>
          <div style="margin-top:var(--space-sm); font-size:var(--font-size-sm);">${items}</div>
        </div>`;
      // Refresh the page data after trigger.
      setTimeout(() => renderReflection(container), 1500);
    } catch (err) {
      triggerResult.innerHTML = `<div style="color:#991b1b;">Error: ${err.message}</div>`;
    } finally {
      triggerBtn.disabled = false;
      triggerBtn.textContent = 'Run Reflection';
    }
  });

  // Filter handling
  const typeFilter = document.getElementById('cycle-type-filter');
  const statusFilter = document.getElementById('cycle-status-filter');
  const onFilter = async () => {
    const params = { limit: 20, offset: 0 };
    if (typeFilter.value) params.type = typeFilter.value;
    if (statusFilter.value) params.status = statusFilter.value;
    try {
      const result = await api.reflectCycles(params);
      document.getElementById('cycles-table').innerHTML = renderCyclesTable(result?.cycles || []);
    } catch { /* ok */ }
  };
  typeFilter.addEventListener('change', onFilter);
  statusFilter.addEventListener('change', onFilter);
}

function formatTimestamp(ts) {
  if (!ts) return '--';
  try {
    const d = new Date(ts);
    return d.toLocaleString();
  } catch {
    return ts;
  }
}

function renderCyclesTable(cycles) {
  if (!cycles || cycles.length === 0) {
    return '<div class="empty-state">No cycles found</div>';
  }

  const rows = cycles.map(c => {
    const props = c.properties || {};
    const signals = props.signalCount ?? props.topSignals?.length ?? '--';
    const status = props.humanFeedbackStatus || props.status || 'PENDING';
    const cycleType = props.type || '--';

    return `
      <tr>
        <td class="mono">${c.id}</td>
        <td>${badge(cycleType, 'info')}</td>
        <td>${badge(status)}</td>
        <td>${signals}</td>
        <td>${c.createdAt ? new Date(c.createdAt).toLocaleString() : '--'}</td>
      </tr>
    `;
  }).join('');

  return `
    <table class="data-table">
      <thead>
        <tr>
          <th>Cycle ID</th>
          <th>Type</th>
          <th>Status</th>
          <th>Signals</th>
          <th>Created</th>
        </tr>
      </thead>
      <tbody>${rows}</tbody>
    </table>
  `;
}

function renderPipelineChart(pipeline) {
  const el = document.getElementById('pipeline-chart');
  if (!el || typeof echarts === 'undefined') return;

  const labels = Object.keys(pipeline);
  const values = Object.values(pipeline);

  if (labels.length === 0) {
    el.innerHTML = '<div class="empty-state">No pipeline data</div>';
    return;
  }

  const colors = {
    PENDING: '#94a3b8', NEEDS_FEEDBACK: '#d97706', REQUESTED: '#f59e0b',
    RECEIVED: '#059669', WAIVED: '#64748b',
  };

  const chart = echarts.init(el);
  chart.setOption({
    tooltip: { trigger: 'item' },
    series: [{
      type: 'pie',
      radius: ['40%', '70%'],
      data: labels.map((l, i) => ({
        name: l,
        value: values[i],
        itemStyle: { color: colors[l] || '#94a3b8' },
      })),
      label: { fontSize: 11 },
    }],
  });
}

function renderTypeChart(byType) {
  const el = document.getElementById('type-chart');
  if (!el || typeof echarts === 'undefined') return;

  const labels = Object.keys(byType);
  const values = Object.values(byType);

  if (labels.length === 0) {
    el.innerHTML = '<div class="empty-state">No type data</div>';
    return;
  }

  const chart = echarts.init(el);
  chart.setOption({
    tooltip: { trigger: 'axis' },
    grid: { left: 80, right: 20, top: 10, bottom: 30 },
    xAxis: { type: 'value' },
    yAxis: { type: 'category', data: labels, axisLabel: { fontSize: 11 } },
    series: [{
      type: 'bar',
      data: values,
      itemStyle: { color: '#6366f1', borderRadius: [0, 3, 3, 0] },
      barMaxWidth: 24,
    }],
  });
}

function renderScoreChart(avgScores) {
  const el = document.getElementById('score-chart');
  if (!el || typeof echarts === 'undefined') return;

  const metrics = ['impact', 'relevance', 'valueContribution'];
  const values = metrics.map(m => avgScores[m] ?? 0);

  const chart = echarts.init(el);
  chart.setOption({
    tooltip: { trigger: 'axis' },
    grid: { left: 120, right: 40, top: 10, bottom: 30 },
    xAxis: { type: 'value', max: 1 },
    yAxis: { type: 'category', data: metrics, axisLabel: { fontSize: 11 } },
    series: [{
      type: 'bar',
      data: values.map((v, i) => ({
        value: v,
        itemStyle: { color: ['#3b82f6', '#059669', '#d97706'][i] },
      })),
      barMaxWidth: 28,
      label: {
        show: true,
        position: 'right',
        formatter: (p) => p.value.toFixed(2),
        fontSize: 11,
      },
    }],
  });
}
