// Node Detail Panel Component

export function renderNodeDetail(node, onClose) {
  if (!node) return '';

  const props = node.properties || {};
  const propRows = Object.entries(props).map(([k, v]) => {
    const displayVal = typeof v === 'object' ? JSON.stringify(v) : String(v);
    return `
      <div class="field-row">
        <span class="prop-key">${k}</span>
        <span class="prop-value">${displayVal}</span>
      </div>
    `;
  }).join('');

  return `
    <div class="detail-header">
      <span class="detail-title">${node.label || 'Node'}</span>
      <button class="detail-close" id="detail-close">&times;</button>
    </div>
    <div class="detail-section">
      <div class="detail-section-title">Identity</div>
      <div class="field-row">
        <span class="prop-key">ID</span>
        <span class="prop-value">${node.id}</span>
      </div>
      <div class="field-row">
        <span class="prop-key">Label</span>
        <span class="prop-value">${node.label}</span>
      </div>
      <div class="field-row">
        <span class="prop-key">Created</span>
        <span class="prop-value">${node.createdAt ? new Date(node.createdAt).toLocaleString() : '--'}</span>
      </div>
    </div>
    ${propRows ? `
      <div class="detail-section">
        <div class="detail-section-title">Properties</div>
        ${propRows}
      </div>
    ` : ''}
  `;
}
