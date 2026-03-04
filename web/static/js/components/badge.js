// Status Badge Component

const STATUS_MAP = {
  'COMPLETED':      'success',
  'RUNNING':        'info',
  'PENDING':        'neutral',
  'NEEDS_FEEDBACK': 'warning',
  'REQUESTED':      'warning',
  'RECEIVED':       'success',
  'WAIVED':         'neutral',
  'FAILED':         'error',
  'OK':             'success',
  'ERROR':          'error',
};

export function badge(text, variant) {
  const cls = variant || STATUS_MAP[text] || 'neutral';
  return `<span class="badge badge-${cls}">${text}</span>`;
}
