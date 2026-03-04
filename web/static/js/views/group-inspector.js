// Group Inspector View — Skills per agent and evolution over time

import { api } from '../api.js';
import { metricCard } from '../components/card.js';
import { dataTable } from '../components/table.js';
import { badge } from '../components/badge.js';

let activeWindow = 'day';
let activeAgent = ''; // empty = all agents

export async function renderGroupInspector(container) {
  container.innerHTML = '<div class="loading">Loading skill data...</div>';

  const params = { window: activeWindow };
  if (activeAgent) params.agent = activeAgent;

  const data = await api.skillsByAgent(params).catch(() => ({
    agents: [],
    skillRoster: [],
  }));

  const totalAgents = data.agents.length;
  const allSkills = new Set();
  let totalInvocations = 0;
  let totalHistoryEvents = 0;

  for (const agent of data.agents) {
    for (const s of (agent.declaredSkills || [])) allSkills.add(s);
    for (const u of (agent.skillUsage || [])) {
      allSkills.add(u.skill);
      totalInvocations += u.totalUses;
    }
    totalHistoryEvents += (agent.skillHistory || []).length;
  }

  // Build skill list sorted
  const skillList = [...allSkills].sort();

  // Build usage lookup: agent -> skill -> count
  const usageLookup = {};
  const declaredLookup = {};
  for (const agent of data.agents) {
    usageLookup[agent.name] = {};
    declaredLookup[agent.name] = new Set(agent.declaredSkills || []);
    for (const u of (agent.skillUsage || [])) {
      usageLookup[agent.name][u.skill] = u.totalUses;
    }
  }

  // Collect all agent names for the filter (need full list, not filtered)
  let allAgentNames = data.agents.map(a => a.name);
  if (activeAgent) {
    // Fetch the full list for the selector
    const fullData = await api.skillsByAgent({ window: activeWindow }).catch(() => ({ agents: [] }));
    allAgentNames = fullData.agents.map(a => a.name);
  }

  container.innerHTML = `
    <div class="card-grid">
      ${metricCard('Agents', totalAgents, activeAgent ? 'filtered' : 'all')}
      ${metricCard('Active Skills', allSkills.size)}
      ${metricCard('Invocations', totalInvocations.toLocaleString())}
      ${metricCard('Roster Events', totalHistoryEvents)}
    </div>

    <div class="card">
      <div class="section-header">
        <h3 class="section-title">Agent-Skill Matrix</h3>
        <div class="section-controls">
          <select id="agent-filter" class="select-sm">
            <option value="">All Agents</option>
            ${allAgentNames.map(n =>
              `<option value="${n}"${n === activeAgent ? ' selected' : ''}>${n}</option>`
            ).join('')}
          </select>
        </div>
      </div>
      <div class="table-scroll">
        ${renderMatrix(data.agents, skillList, usageLookup, declaredLookup)}
      </div>
    </div>

    <div class="card">
      <div class="section-header">
        <h3 class="section-title">Skill Evolution History</h3>
      </div>
      ${renderSkillHistory(data.agents)}
    </div>

    <div class="card">
      <div class="section-header">
        <h3 class="section-title">Skill Usage Timeline</h3>
        <div class="section-controls" id="window-controls">
          ${['hour', 'day', 'week'].map(w =>
            `<button class="btn btn-sm${w === activeWindow ? ' btn-active' : ''}" data-window="${w}">${w}</button>`
          ).join('')}
        </div>
      </div>
      <div id="timeline-chart" class="chart-container chart-container-md"></div>
    </div>

    ${dataTable(
      [
        { label: 'Skill', key: 'skill' },
        { label: 'Agents', render: r => (r.agents || []).map(a => badge(a, 'info')).join(' ') },
        { label: 'Total Uses', key: 'totalUses' },
      ],
      data.skillRoster,
      { title: 'Skill Roster' }
    )}
  `;

  injectStyles();

  // Agent filter handler
  document.getElementById('agent-filter')?.addEventListener('change', (e) => {
    activeAgent = e.target.value;
    renderGroupInspector(container);
  });

  // Window selector handler
  document.getElementById('window-controls')?.addEventListener('click', (e) => {
    const btn = e.target.closest('[data-window]');
    if (btn) {
      activeWindow = btn.dataset.window;
      renderGroupInspector(container);
    }
  });

  // Render timeline chart
  renderTimelineChart('timeline-chart', data.agents, activeWindow);
}

function renderMatrix(agents, skillList, usageLookup, declaredLookup) {
  if (agents.length === 0 || skillList.length === 0) {
    return '<div class="empty-state">No agent-skill data available</div>';
  }

  const headerCells = skillList.map(s =>
    `<th class="matrix-skill-header">${s}</th>`
  ).join('');

  const rows = agents.map(agent => {
    const cells = skillList.map(skill => {
      const used = usageLookup[agent.name]?.[skill] || 0;
      const declared = declaredLookup[agent.name]?.has(skill);

      if (used > 0) {
        return `<td>${badge(String(used), 'success')}</td>`;
      } else if (declared) {
        return `<td>${badge('declared', 'info')}</td>`;
      }
      return '<td></td>';
    }).join('');
    return `<tr><td class="mono">${agent.name}</td>${cells}</tr>`;
  }).join('');

  return `
    <table class="data-table matrix-table">
      <thead><tr><th>Agent</th>${headerCells}</tr></thead>
      <tbody>${rows}</tbody>
    </table>
  `;
}

function renderSkillHistory(agents) {
  // Flatten all history events across agents into a single timeline
  const events = [];
  for (const agent of agents) {
    for (const h of (agent.skillHistory || [])) {
      events.push({ agent: agent.name, ...h });
    }
  }

  if (events.length === 0) {
    return '<div class="empty-state">No roster evolution events recorded</div>';
  }

  // Sort by declaredAt ascending
  events.sort((a, b) => (a.declaredAt || '').localeCompare(b.declaredAt || ''));

  const columns = [
    { label: 'Time', render: r => r.declaredAt ? formatTime(r.declaredAt) : '--', mono: true },
    { label: 'Agent', key: 'agent' },
    { label: 'Skill', key: 'skill' },
    { label: 'Version', render: r => r.rosterVersion ? `v${r.rosterVersion}` : '--', mono: true },
    { label: 'Status', render: r => {
      if (r.removedAt) return badge('removed', 'error');
      return badge('active', 'success');
    }},
    { label: 'Removed At', render: r => r.removedAt ? formatTime(r.removedAt) : '', mono: true },
  ];

  return dataTable(columns, events);
}

function formatTime(iso) {
  const d = new Date(iso);
  return d.toLocaleString(undefined, {
    month: 'short', day: 'numeric',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
  });
}

function renderTimelineChart(elementId, agents, windowSize) {
  const el = document.getElementById(elementId);
  if (!el || typeof echarts === 'undefined') return;

  // Collect all time buckets and per-agent series
  const allBuckets = new Set();
  const seriesData = {};

  for (const agent of agents) {
    seriesData[agent.name] = {};
    for (const usage of (agent.skillUsage || [])) {
      for (const t of (usage.timeline || [])) {
        allBuckets.add(t.time);
        seriesData[agent.name][t.time] = (seriesData[agent.name][t.time] || 0) + t.count;
      }
    }
  }

  const buckets = [...allBuckets].sort();

  if (buckets.length === 0) {
    el.innerHTML = '<div class="empty-state">No usage timeline data</div>';
    return;
  }

  const COLORS = ['#3b82f6', '#059669', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899', '#06b6d4', '#84cc16'];

  const series = agents
    .filter(a => (a.skillUsage || []).length > 0)
    .map((agent, i) => ({
      name: agent.name,
      type: 'bar',
      stack: 'total',
      data: buckets.map(b => seriesData[agent.name]?.[b] || 0),
      itemStyle: { color: COLORS[i % COLORS.length] },
    }));

  const formatLabel = (val) => {
    const d = new Date(val);
    if (windowSize === 'hour') return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit' });
    return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
  };

  const chart = echarts.init(el);
  chart.setOption({
    tooltip: { trigger: 'axis' },
    legend: { bottom: 0, type: 'scroll' },
    grid: { left: 50, right: 20, top: 10, bottom: 40 },
    xAxis: {
      type: 'category',
      data: buckets.map(formatLabel),
      axisLabel: { fontSize: 10, rotate: buckets.length > 10 ? 30 : 0 },
    },
    yAxis: { type: 'value', name: 'Invocations', minInterval: 1 },
    series: series,
  });

  globalThis.addEventListener('resize', () => chart.resize(), { once: true });
}

function injectStyles() {
  if (document.getElementById('group-inspector-styles')) return;
  const style = document.createElement('style');
  style.id = 'group-inspector-styles';
  style.textContent = `
    .table-scroll { overflow-x: auto; }
    .matrix-table th, .matrix-table td { text-align: center; min-width: 80px; }
    .matrix-table td:first-child { text-align: left; }
    .matrix-skill-header { font-size: var(--font-size-xs, 11px); writing-mode: horizontal-tb; }
    .section-controls { display: flex; gap: var(--space-xs, 4px); align-items: center; }
    .btn-sm { padding: 2px 10px; font-size: var(--font-size-xs, 11px); border: 1px solid var(--color-border, #333); border-radius: var(--radius-sm, 4px); background: transparent; color: var(--color-text, #ccc); cursor: pointer; }
    .btn-sm:hover { background: var(--color-hover, #2a2a2a); }
    .btn-active { background: var(--color-accent, #3b82f6); color: #fff; border-color: var(--color-accent, #3b82f6); }
    .select-sm { padding: 2px 8px; font-size: var(--font-size-xs, 11px); border: 1px solid var(--color-border, #333); border-radius: var(--radius-sm, 4px); background: var(--color-surface, #1a1a1a); color: var(--color-text, #ccc); cursor: pointer; }
    .chart-container-md { height: 300px; }
  `;
  document.head.appendChild(style);
}
