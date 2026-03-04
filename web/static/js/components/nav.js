// Sidebar Navigation Component

const NAV_ITEMS = [
  { id: 'dashboard',     label: 'Dashboard',      icon: '\u25A3' },
  { id: 'graph-browser', label: 'Graph Browser',   icon: '\u25CE' },
  { id: 'data-stats',    label: 'Data Stats',      icon: '\u25A4' },
  { id: 'config',        label: 'Configuration',   icon: '\u2699' },
  { id: 'reflection',    label: 'Reflection',      icon: '\u25C8' },
  { id: 'group-inspector', label: 'Group Inspector', icon: '\u2726' },
  { id: 'compliance',      label: 'Compliance',      icon: '\u2611' },
];

export function renderNav(container, activeView, onNavigate) {
  container.innerHTML = `
    <div class="sidebar-logo">
      <h1>KafGraph</h1>
      <div class="version" id="sidebar-version">loading...</div>
    </div>
    <nav class="sidebar-nav">
      ${NAV_ITEMS.map(item => `
        <a class="nav-link${item.id === activeView ? ' active' : ''}"
           href="#/${item.id}" data-view="${item.id}">
          <span class="nav-icon">${item.icon}</span>
          <span class="nav-label">${item.label}</span>
        </a>
      `).join('')}
    </nav>
    <div class="sidebar-status" id="sidebar-status">
      <div class="status-row">
        <span>Status</span>
        <span class="status-value" id="status-health">--</span>
      </div>
      <div class="status-row">
        <span>Nodes</span>
        <span class="status-value" id="status-nodes">--</span>
      </div>
      <div class="status-row">
        <span>Uptime</span>
        <span class="status-value" id="status-uptime">--</span>
      </div>
    </div>
  `;

  // Add nav link styles
  if (!document.getElementById('nav-styles')) {
    const style = document.createElement('style');
    style.id = 'nav-styles';
    style.textContent = `
      .nav-link {
        display: flex;
        align-items: center;
        gap: var(--space-sm);
        padding: var(--space-sm) var(--space-md);
        margin: 2px var(--space-sm);
        border-radius: var(--radius-sm);
        color: var(--color-sidebar-text);
        text-decoration: none;
        font-size: var(--font-size-sm);
        transition: all 0.15s;
      }
      .nav-link:hover {
        background: var(--color-sidebar-hover);
        color: var(--color-sidebar-active);
      }
      .nav-link.active {
        background: var(--color-sidebar-hover);
        color: var(--color-sidebar-active);
        font-weight: 500;
      }
      .nav-icon {
        width: 20px;
        text-align: center;
        font-size: var(--font-size-lg);
      }
    `;
    document.head.appendChild(style);
  }

  container.querySelectorAll('.nav-link').forEach(link => {
    link.addEventListener('click', (e) => {
      e.preventDefault();
      const view = link.dataset.view;
      window.location.hash = '#/' + view;
    });
  });
}

export function updateSidebarStatus(info, stats) {
  const v = document.getElementById('sidebar-version');
  if (v && info) v.textContent = 'v' + info.version;

  const h = document.getElementById('status-health');
  if (h) h.textContent = 'OK';

  const n = document.getElementById('status-nodes');
  if (n && stats) n.textContent = (stats.nodes?.total ?? 0).toLocaleString();

  const u = document.getElementById('status-uptime');
  if (u && info) u.textContent = info.uptime || '--';
}
