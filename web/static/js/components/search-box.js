// Graph Search Input Component

export function searchBox(onSearch) {
  const container = document.createElement('div');
  container.className = 'search-box';
  container.innerHTML = `
    <input type="text" class="search-input" placeholder="Search nodes by ID or property...">
    <button class="btn btn-sm btn-primary search-btn">Search</button>
  `;

  const input = container.querySelector('.search-input');
  const btn = container.querySelector('.search-btn');

  const doSearch = () => {
    const q = input.value.trim();
    if (q) onSearch(q);
  };

  btn.addEventListener('click', doSearch);
  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') doSearch();
  });

  // Inject style once
  if (!document.getElementById('search-box-styles')) {
    const style = document.createElement('style');
    style.id = 'search-box-styles';
    style.textContent = `
      .search-box { display: flex; gap: var(--space-xs); flex: 1; }
      .search-input {
        flex: 1;
        padding: 6px 12px;
        border: 1px solid var(--color-border);
        border-radius: var(--radius-sm);
        font-size: var(--font-size-sm);
        font-family: var(--font-sans);
        outline: none;
      }
      .search-input:focus { border-color: var(--color-accent); }
    `;
    document.head.appendChild(style);
  }

  return container;
}
