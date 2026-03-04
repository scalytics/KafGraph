// KafGraph Management API Client

const BASE = '/api/v2/mgmt';

async function fetchJSON(path) {
  const res = await fetch(BASE + path);
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`API ${res.status}: ${body}`);
  }
  return res.json();
}

async function postJSON(path, body) {
  const res = await fetch(BASE + path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`API ${res.status}: ${text}`);
  }
  return res.json();
}

// --- Compliance API helpers ---
const COMP = '/api/v2/compliance';

async function fetchComplianceJSON(path) {
  const res = await fetch(COMP + path);
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`API ${res.status}: ${body}`);
  }
  return res.json();
}

async function postComplianceJSON(path, body) {
  const res = await fetch(COMP + path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`API ${res.status}: ${text}`);
  }
  return res.json();
}

async function putComplianceJSON(path, body) {
  const res = await fetch(COMP + path, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`API ${res.status}: ${text}`);
  }
  return res.json();
}

async function deleteCompliance(path) {
  const res = await fetch(COMP + path, { method: 'DELETE' });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`API ${res.status}: ${text}`);
  }
  return res.json();
}

export const api = {
  info:           () => fetchJSON('/info'),
  storage:        () => fetchJSON('/storage'),
  graphStats:     () => fetchJSON('/stats/graph'),
  graphExplore:   (params) => fetchJSON('/graph/explore?' + new URLSearchParams(params)),
  graphSearch:    (params) => fetchJSON('/graph/search?' + new URLSearchParams(params)),
  config:         () => fetchJSON('/config'),
  configDetailed: () => fetchJSON('/config/detailed'),
  cluster:        () => fetchJSON('/cluster'),
  reflectSummary: () => fetchJSON('/reflect/summary'),
  reflectCycles:  (params) => fetchJSON('/reflect/cycles?' + new URLSearchParams(params)),
  reflectTrigger: (body) => postJSON('/reflect/trigger', body),
  activity:       (params) => fetchJSON('/activity?' + new URLSearchParams(params)),
  skillsByAgent:  (params) => fetchJSON('/skills/by-agent?' + new URLSearchParams(params || {})),

  // Compliance
  complianceDashboard:  () => fetchComplianceJSON('/dashboard'),
  complianceFrameworks: () => fetchComplianceJSON('/frameworks'),
  complianceRules:      (params) => fetchComplianceJSON('/rules?' + new URLSearchParams(params || {})),
  complianceScan:       (body) => postComplianceJSON('/scan', body),
  complianceScans:      () => fetchComplianceJSON('/scans'),
  complianceScanDetail: (id) => fetchComplianceJSON('/scans/' + id),
  complianceScore:      () => fetchComplianceJSON('/score'),

  // GDPR
  gdprSetup:            () => fetchComplianceJSON('/gdpr/setup'),
  gdprSetupUpdate:      (body) => putComplianceJSON('/gdpr/setup', body),
  gdprRopa:             () => fetchComplianceJSON('/gdpr/ropa'),
  gdprRopaCreate:       (body) => postComplianceJSON('/gdpr/ropa', body),
  gdprRopaUpdate:       (id, body) => putComplianceJSON('/gdpr/ropa/' + id, body),
  gdprDsr:              () => fetchComplianceJSON('/gdpr/dsr'),
  gdprDsrCreate:        (body) => postComplianceJSON('/gdpr/dsr', body),
  gdprDsrSla:           () => fetchComplianceJSON('/gdpr/dsr/sla'),
  gdprBreaches:         () => fetchComplianceJSON('/gdpr/breaches'),
  gdprBreachCreate:     (body) => postComplianceJSON('/gdpr/breaches', body),
  gdprDpia:             () => fetchComplianceJSON('/gdpr/dpia'),
  gdprDpiaCreate:       (body) => postComplianceJSON('/gdpr/dpia', body),
  gdprProcessors:       () => fetchComplianceJSON('/gdpr/processors'),
  gdprChecklist:        () => fetchComplianceJSON('/gdpr/checklist'),
  gdprEvidence:         () => fetchComplianceJSON('/gdpr/evidence'),
  gdprDataCategories:   () => fetchComplianceJSON('/gdpr/data-categories'),
  gdprLegalBases:       () => fetchComplianceJSON('/gdpr/legal-bases'),
  gdprSecurityMeasures: () => fetchComplianceJSON('/gdpr/security-measures'),

  // Inspections
  inspections:          () => fetchComplianceJSON('/inspections'),
  inspectionDetail:     (id) => fetchComplianceJSON('/inspections/' + id),
  inspectionCreate:     (body) => postComplianceJSON('/inspections', body),
  inspectionUpdate:     (id, body) => putComplianceJSON('/inspections/' + id, body),
  inspectionSignOff:    (id, body) => postComplianceJSON('/inspections/' + id + '/sign-off', body),
  findingCreate:        (inspId, body) => postComplianceJSON('/inspections/' + inspId + '/findings', body),
  findingDetail:        (id) => fetchComplianceJSON('/findings/' + id),
  findingUpdate:        (id, body) => putComplianceJSON('/findings/' + id, body),
  remediationCreate:    (findingId, body) => postComplianceJSON('/findings/' + findingId + '/remediation', body),
  remediationUpdate:    (id, body) => putComplianceJSON('/remediation/' + id, body),

  // Data Flows
  dataFlows:            () => fetchComplianceJSON('/gdpr/data-flows'),
  dataFlowDetail:       (id) => fetchComplianceJSON('/gdpr/data-flows/' + id),
  dataFlowCreate:       (body) => postComplianceJSON('/gdpr/data-flows', body),
  dataFlowUpdate:       (id, body) => putComplianceJSON('/gdpr/data-flows/' + id, body),
  dataFlowDelete:       (id) => deleteCompliance('/gdpr/data-flows/' + id),
  dataFlowMap:          () => fetchComplianceJSON('/gdpr/data-flows/map'),
  dataFlowValidate:     (body) => postComplianceJSON('/gdpr/data-flows/validate', body || {}),

  // Audit Trail
  complianceEvents:     (params) => fetchComplianceJSON('/events?' + new URLSearchParams(params || {})),
};
