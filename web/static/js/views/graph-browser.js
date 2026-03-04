// Graph Browser View — Cytoscape.js integration

import { api } from '../api.js';
import { searchBox } from '../components/search-box.js';
import { renderNodeDetail } from '../components/node-detail.js';

const NODE_COLORS = {
  Agent:           '#3b82f6',
  Conversation:    '#6366f1',
  Message:         '#64748b',
  LearningSignal:  '#059669',
  ReflectionCycle: '#d97706',
  HumanFeedback:   '#7c3aed',
  Skill:           '#0891b2',
  SharedMemory:    '#475569',
  AuditEvent:      '#dc2626',
};

let cy = null;
let currentLayout = 'cose';

export async function renderGraphBrowser(container) {
  container.innerHTML = `
    <div class="graph-container" id="graph-container">
      <div class="toolbar graph-toolbar">
        <div id="search-slot"></div>
        <select class="form-select" id="label-filter">
          <option value="">All labels</option>
        </select>
        <select class="form-select" id="depth-select">
          <option value="1">Depth 1</option>
          <option value="2">Depth 2</option>
        </select>
        <div class="layout-selector">
          <button class="layout-btn active" data-layout="cose">Force</button>
          <button class="layout-btn" data-layout="concentric">Concentric</button>
          <button class="layout-btn" data-layout="breadthfirst">Tree</button>
        </div>
      </div>
      <div class="graph-canvas">
        <div id="cy"></div>
      </div>
      <div class="graph-detail" id="node-detail" style="display:none;"></div>
    </div>
    <div class="graph-legend" id="graph-legend"></div>
  `;

  // Insert search box
  const searchSlot = document.getElementById('search-slot');
  searchSlot.appendChild(searchBox(doSearch));

  // Populate label filter
  try {
    const stats = await api.graphStats();
    const labelFilter = document.getElementById('label-filter');
    for (const label of Object.keys(stats.nodes?.byLabel ?? {})) {
      const opt = document.createElement('option');
      opt.value = label;
      opt.textContent = label;
      labelFilter.appendChild(opt);
    }
  } catch { /* non-critical */ }

  // Layout buttons
  document.querySelectorAll('.layout-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.layout-btn').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      currentLayout = btn.dataset.layout;
      if (cy) runLayout();
    });
  });

  // Label filter
  document.getElementById('label-filter').addEventListener('change', (e) => {
    const label = e.target.value;
    if (label) {
      loadByLabel(label);
    }
  });

  // Render legend
  renderLegend();

  // Init Cytoscape
  initCytoscape();

  // Listen for global search events (when already on graph-browser)
  const searchHandler = (evt) => {
    const q = evt.detail;
    const searchInput = container.querySelector('.search-input');
    if (searchInput) searchInput.value = q;
    doSearch(q);
  };
  window.addEventListener('kafgraph-search', searchHandler);

  // Check for pending global search
  if (window.__kafgraphSearch) {
    const q = window.__kafgraphSearch;
    delete window.__kafgraphSearch;
    const searchInput = container.querySelector('.search-input');
    if (searchInput) searchInput.value = q;
    doSearch(q);
  } else {
    // Load initial data — sample first available label
    try {
      const stats = await api.graphStats();
      const labels = Object.keys(stats.nodes?.byLabel ?? {});
      if (labels.length > 0) {
        loadByLabel(labels[0]);
      }
    } catch { /* empty canvas is fine */ }
  }
}

function initCytoscape() {
  if (typeof cytoscape === 'undefined') {
    document.getElementById('cy').innerHTML =
      '<div class="empty-state" style="padding-top:120px">Cytoscape.js not loaded</div>';
    return;
  }

  cy = cytoscape({
    container: document.getElementById('cy'),
    style: [
      {
        selector: 'node',
        style: {
          'label': 'data(shortLabel)',
          'background-color': 'data(color)',
          'color': '#1e293b',
          'font-size': '10px',
          'font-family': '-apple-system, BlinkMacSystemFont, sans-serif',
          'text-valign': 'bottom',
          'text-margin-y': 6,
          'width': 28,
          'height': 28,
          'border-width': 2,
          'border-color': '#ffffff',
        }
      },
      {
        selector: 'edge',
        style: {
          'label': 'data(label)',
          'width': 1.5,
          'line-color': '#cbd5e1',
          'target-arrow-color': '#94a3b8',
          'target-arrow-shape': 'triangle',
          'arrow-scale': 0.8,
          'curve-style': 'bezier',
          'font-size': '8px',
          'color': '#94a3b8',
          'text-rotation': 'autorotate',
        }
      },
      {
        selector: 'node:selected',
        style: {
          'border-width': 3,
          'border-color': '#3b82f6',
        }
      },
    ],
    layout: { name: 'cose' },
    wheelSensitivity: 0.3,
  });

  // Click handler
  cy.on('tap', 'node', (evt) => {
    const data = evt.target.data();
    showNodeDetail(data);
  });

  cy.on('tap', (evt) => {
    if (evt.target === cy) {
      hideNodeDetail();
    }
  });

  // Double-click to re-explore
  cy.on('dbltap', 'node', (evt) => {
    const nodeId = evt.target.data('nodeId');
    if (nodeId) loadByNode(nodeId);
  });
}

async function doSearch(query) {
  const cyEl = document.getElementById('cy');
  if (cyEl) {
    cyEl.innerHTML = '<div class="empty-state" style="padding-top:120px">Searching\u2026</div>';
  }
  try {
    const result = await api.graphSearch({ q: query, limit: 30 });
    const hasNodes = result.nodes && result.nodes.length > 0;
    if (!hasNodes) {
      if (cyEl) {
        cyEl.innerHTML = '<div class="empty-state" style="padding-top:120px">No results found</div>';
      }
      return;
    }
    loadGraphData(result);
  } catch (err) {
    console.error('Search error:', err);
    if (cyEl) {
      cyEl.innerHTML = `<div class="empty-state" style="padding-top:120px">Search failed: ${err.message}</div>`;
    }
  }
}

async function loadByLabel(label) {
  try {
    const result = await api.graphExplore({ label, limit: 50 });
    loadGraphData(result);
  } catch (err) {
    console.error('Explore error:', err);
  }
}

async function loadByNode(nodeId) {
  const depth = document.getElementById('depth-select')?.value || '1';
  try {
    const result = await api.graphExplore({ nodeId, depth, limit: 50 });
    loadGraphData(result);
  } catch (err) {
    console.error('Explore error:', err);
  }
}

function loadGraphData(data) {
  if (!cy) return;

  cy.elements().remove();

  const nodes = (data.nodes || []).map(n => ({
    data: {
      id: n.id,
      nodeId: n.id,
      label: n.label,
      shortLabel: shortenId(n.id),
      color: NODE_COLORS[n.label] || '#64748b',
      properties: n.properties,
      createdAt: n.createdAt,
    }
  }));

  const edges = (data.edges || []).filter(e => {
    const hasSource = nodes.some(n => n.data.id === e.fromId);
    const hasTarget = nodes.some(n => n.data.id === e.toId);
    return hasSource && hasTarget;
  }).map(e => ({
    data: {
      id: e.id,
      source: e.fromId,
      target: e.toId,
      label: e.label,
    }
  }));

  cy.add([...nodes, ...edges]);
  runLayout();
}

function runLayout() {
  if (!cy) return;
  cy.layout({
    name: currentLayout,
    animate: true,
    animationDuration: 300,
    fit: true,
    padding: 40,
    nodeRepulsion: 8000,
    idealEdgeLength: 80,
  }).run();
}

function showNodeDetail(data) {
  const panel = document.getElementById('node-detail');
  const graphContainer = document.getElementById('graph-container');
  if (!panel) return;

  panel.style.display = 'block';
  graphContainer.classList.add('detail-open');
  panel.innerHTML = renderNodeDetail(data);

  document.getElementById('detail-close')?.addEventListener('click', hideNodeDetail);
}

function hideNodeDetail() {
  const panel = document.getElementById('node-detail');
  const graphContainer = document.getElementById('graph-container');
  if (panel) panel.style.display = 'none';
  if (graphContainer) graphContainer.classList.remove('detail-open');
}

function shortenId(id) {
  if (!id) return '';
  const parts = id.split(':');
  if (parts.length >= 3) return parts[1] + ':' + parts[2];
  return id.length > 16 ? id.slice(0, 16) + '...' : id;
}

function renderLegend() {
  const el = document.getElementById('graph-legend');
  if (!el) return;
  el.innerHTML = Object.entries(NODE_COLORS).map(([label, color]) => `
    <span class="legend-item">
      <span class="legend-dot" style="background:${color}"></span>
      ${label}
    </span>
  `).join('');
}
