// KafGraph Management UI — App Shell & Hash Router

import { api } from './api.js';
import { renderNav, updateSidebarStatus } from './components/nav.js';
import { renderDashboard } from './views/dashboard.js';
import { renderGraphBrowser } from './views/graph-browser.js';
import { renderDataStats } from './views/data-stats.js';
import { renderConfig } from './views/config.js';
import { renderReflection } from './views/reflection.js';
import { renderGroupInspector } from './views/group-inspector.js';
import { renderCompliance } from './views/compliance.js';

const VIEW_TITLES = {
  'dashboard':        'Dashboard',
  'graph-browser':    'Graph Browser',
  'data-stats':       'Data Stats',
  'config':           'Configuration',
  'reflection':       'Reflection',
  'group-inspector':  'Group Inspector',
  'compliance':       'Compliance',
};

const VIEWS = {
  'dashboard':        renderDashboard,
  'graph-browser':    renderGraphBrowser,
  'data-stats':       renderDataStats,
  'config':           renderConfig,
  'reflection':       renderReflection,
  'group-inspector':  renderGroupInspector,
  'compliance':       renderCompliance,
};

let currentView = null;
let refreshTimer = null;

function getViewFromHash() {
  const hash = window.location.hash.replace('#/', '').replace('#', '');
  return hash && VIEWS[hash] ? hash : 'dashboard';
}

async function switchView(viewId) {
  if (refreshTimer) {
    clearInterval(refreshTimer);
    refreshTimer = null;
  }

  currentView = viewId;
  const content = document.getElementById('content');
  const title = document.getElementById('topbar-title');
  const sidebar = document.getElementById('sidebar');

  title.textContent = VIEW_TITLES[viewId] || viewId;
  content.innerHTML = '<div class="loading">Loading...</div>';

  renderNav(sidebar, viewId);

  try {
    await VIEWS[viewId](content);
  } catch (err) {
    content.innerHTML = `<div class="card"><p>Error loading view: ${err.message}</p></div>`;
    console.error('View error:', err);
  }

  loadSidebarStatus();
}

async function loadSidebarStatus() {
  try {
    const [info, stats] = await Promise.all([
      api.info().catch(() => null),
      api.graphStats().catch(() => null),
    ]);
    updateSidebarStatus(info, stats);
  } catch {
    // sidebar status is non-critical
  }
}

function onHashChange() {
  const viewId = getViewFromHash();
  if (viewId !== currentView) {
    switchView(viewId);
  }
}

// Global search handler
function setupGlobalSearch() {
  const input = document.getElementById('global-search');
  if (!input) return;
  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      const q = input.value.trim();
      if (!q) return;
      if (currentView === 'graph-browser') {
        // Already on graph-browser — dispatch search event directly
        window.dispatchEvent(new CustomEvent('kafgraph-search', { detail: q }));
      } else {
        window.__kafgraphSearch = q;
        window.location.hash = '#/graph-browser';
      }
    }
  });
}

// Init
window.addEventListener('hashchange', onHashChange);
document.addEventListener('DOMContentLoaded', () => {
  setupGlobalSearch();
  if (!window.location.hash) {
    window.location.hash = '#/dashboard';
  } else {
    onHashChange();
  }
});
