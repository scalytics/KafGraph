// Copyright 2026 Scalytics, Inc.
// Copyright 2026 Mirko Kämpf
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/scalytics/kafgraph/internal/brain"
	"github.com/scalytics/kafgraph/internal/config"
	"github.com/scalytics/kafgraph/internal/graph"
)

// registerManagementRoutes wires all management API routes onto the mux.
func registerManagementRoutes(mux *http.ServeMux, g *graph.Graph, opts *serverOpts) {
	mux.HandleFunc("GET /api/v2/mgmt/info", handleMgmtInfo(opts))
	mux.HandleFunc("GET /api/v2/mgmt/storage", handleMgmtStorage(g))
	mux.HandleFunc("GET /api/v2/mgmt/stats/graph", handleMgmtGraphStats(g))
	mux.HandleFunc("GET /api/v2/mgmt/graph/explore", handleMgmtGraphExplore(g))
	mux.HandleFunc("GET /api/v2/mgmt/graph/search", handleMgmtGraphSearch(g))
	mux.HandleFunc("GET /api/v2/mgmt/config", handleMgmtConfig(opts))
	mux.HandleFunc("GET /api/v2/mgmt/config/detailed", handleMgmtConfigDetailed(opts))
	mux.HandleFunc("GET /api/v2/mgmt/cluster", handleMgmtCluster(opts))
	mux.HandleFunc("GET /api/v2/mgmt/reflect/summary", handleMgmtReflectSummary(g))
	mux.HandleFunc("GET /api/v2/mgmt/reflect/cycles", handleMgmtReflectCycles(g))
	mux.HandleFunc("POST /api/v2/mgmt/reflect/trigger", handleMgmtReflectTrigger(g, opts))
	mux.HandleFunc("GET /api/v2/mgmt/activity", handleMgmtActivity(g))
	mux.HandleFunc("GET /api/v2/mgmt/skills/by-agent", handleMgmtSkillsByAgent(g))
}

// --- Service & Storage ---

func handleMgmtInfo(opts *serverOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		uptime := time.Since(opts.startedAt).Truncate(time.Second).String()
		dataDir := ""
		engine := "badger"
		if opts.cfg != nil {
			dataDir = opts.cfg.DataDir
			engine = opts.cfg.StorageEngine
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"version":       config.Version,
			"commit":        config.GitCommit,
			"built":         config.BuildDate,
			"uptime":        uptime,
			"startedAt":     opts.startedAt.Format(time.RFC3339),
			"storageEngine": engine,
			"dataDir":       dataDir,
			"goVersion":     runtime.Version(),
			"os":            runtime.GOOS,
			"arch":          runtime.GOARCH,
		})
	}
}

func handleMgmtStorage(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		result := map[string]any{
			"engine": "badger",
		}

		// Try to get BadgerDB size info if available.
		type sizer interface {
			Size() (lsm int64, vlog int64)
		}
		type dbAccessor interface {
			DB() interface{ Size() (int64, int64) }
		}

		backend := g.StorageBackend()
		if s, ok := backend.(sizer); ok {
			lsm, vlog := s.Size()
			result["lsmSize"] = lsm
			result["vlogSize"] = vlog
		} else if a, ok := backend.(dbAccessor); ok {
			db := a.DB()
			if db != nil {
				lsm, vlog := db.Size()
				result["lsmSize"] = lsm
				result["vlogSize"] = vlog
			}
		}

		writeJSON(w, http.StatusOK, result)
	}
}

// --- Graph Stats & Exploration ---

// knownLabels is the set of graph node labels used across KafGraph.
var knownLabels = []string{
	"Agent", "Conversation", "Message", "LearningSignal",
	"ReflectionCycle", "HumanFeedback", "Skill", "SharedMemory", "AuditEvent",
	// Compliance engine labels
	"ComplianceFramework", "ComplianceRule", "ComplianceEvaluation", "ComplianceScan",
	// GDPR module labels
	"OrgSetup", "DataCategory", "LegalBasis", "SecurityMeasure",
	"ProcessingActivity", "DataSubjectRequest", "DataBreach",
	"DPIA", "DPIARisk", "DataProcessor", "ChecklistItem", "Evidence",
	// Inspection and data flow labels
	"Inspection", "InspectionFinding", "RemediationAction",
	"DataFlow", "DataFlowValidation", "ComplianceEvent",
}

// knownEdgeLabels is the set of edge labels used across KafGraph.
var knownEdgeLabels = []string{
	"AUTHORED", "BELONGS_TO", "PARTICIPATED_IN", "REPLIED_TO",
	"DECIDED", "LEARNED", "USES_SKILL", "HAS_SKILL",
	"SHARED_MEMORY", "SHARED_BY", "REFERENCES",
	"HAS_SIGNAL", "ROLLUP_OF", "FEEDBACK_ON", "HAS_FEEDBACK", "SCORED",
	"DELEGATES_TO", "REPORTS_TO", "AUDITED_BY",
	// Compliance engine edges
	"DEFINES_RULE", "EVALUATED_BY", "PART_OF_SCAN", "SCOPED_TO",
	// GDPR module edges
	"HAS_LEGAL_BASIS", "PROCESSES_CATEGORY", "PROTECTED_BY", "TRANSFERS_TO",
	"MANAGED_BY", "DSR_FOR_ACTIVITY", "HANDLED_BY",
	"BREACH_AFFECTS", "BREACH_INVOLVES",
	"DPIA_FOR", "HAS_RISK", "MITIGATED_BY",
	"PROCESSES_FOR", "EVIDENCED_BY",
	// Inspection edges
	"BASED_ON", "INSPECTS", "CONDUCTED_BY", "APPROVED_BY",
	"HAS_FINDING", "AFFECTS", "REMEDIATED_BY", "VERIFIED_BY",
	// Data flow edges
	"FROM_ACTIVITY", "TO_ACTIVITY", "TO_PROCESSOR",
	"CARRIES", "GOVERNED_BY", "VALIDATES", "PART_OF_INSPECTION",
	// Audit trail
	"RELATES_TO",
}

func handleMgmtGraphStats(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		nodesByLabel := make(map[string]int)
		totalNodes := 0

		for _, label := range knownLabels {
			count := countByLabel(g, label)
			if count > 0 {
				nodesByLabel[label] = count
				totalNodes += count
			}
		}

		edgesByLabel := make(map[string]int)
		totalEdges := 0

		for _, label := range knownEdgeLabels {
			count := countEdgesByLabel(g, label)
			if count > 0 {
				edgesByLabel[label] = count
				totalEdges += count
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"nodes": map[string]any{
				"total":   totalNodes,
				"byLabel": nodesByLabel,
			},
			"edges": map[string]any{
				"total":   totalEdges,
				"byLabel": edgesByLabel,
			},
		})
	}
}

func handleMgmtGraphExplore(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		nodeID := q.Get("nodeId")
		label := q.Get("label")
		limitStr := q.Get("limit")

		limit := 50
		if limitStr != "" {
			if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 200 {
				limit = v
			}
		}

		if nodeID != "" {
			depthStr := q.Get("depth")
			depth := 1
			if depthStr == "2" {
				depth = 2
			}
			exploreFromNode(w, g, nodeID, depth, limit)
			return
		}

		if label != "" {
			exploreByLabel(w, g, label, limit)
			return
		}

		writeError(w, http.StatusBadRequest, "nodeId or label parameter required")
	}
}

func exploreFromNode(w http.ResponseWriter, g *graph.Graph, nodeID string, depth, limit int) {
	seenNodes := make(map[string]bool)
	seenEdges := make(map[string]bool)
	var nodes []any
	var edges []any

	focal, err := g.GetNode(graph.NodeID(nodeID))
	if err != nil {
		writeError(w, http.StatusNotFound, "node not found: "+nodeID)
		return
	}
	seenNodes[nodeID] = true
	nodes = append(nodes, nodeToJSON(focal))

	// Depth 1: get immediate neighbors
	neighbors, err := g.Neighbors(graph.NodeID(nodeID))
	if err != nil {
		neighbors = nil
	}

	for _, e := range neighbors {
		if len(nodes)+len(edges) >= limit*2 {
			break
		}
		if !seenEdges[string(e.ID)] {
			seenEdges[string(e.ID)] = true
			edges = append(edges, edgeToJSON(e))
		}
		for _, nid := range []graph.NodeID{e.FromID, e.ToID} {
			if !seenNodes[string(nid)] {
				n, err := g.GetNode(nid)
				if err == nil {
					seenNodes[string(nid)] = true
					nodes = append(nodes, nodeToJSON(n))
				}
			}
		}
	}

	// Depth 2: expand neighbors
	if depth >= 2 {
		expandIDs := make([]string, 0)
		for id := range seenNodes {
			if id != nodeID {
				expandIDs = append(expandIDs, id)
			}
		}
		for _, id := range expandIDs {
			if len(nodes) >= limit {
				break
			}
			nEdges, err := g.Neighbors(graph.NodeID(id))
			if err != nil {
				continue
			}
			for _, e := range nEdges {
				if len(nodes) >= limit {
					break
				}
				if !seenEdges[string(e.ID)] {
					seenEdges[string(e.ID)] = true
					edges = append(edges, edgeToJSON(e))
				}
				for _, nid := range []graph.NodeID{e.FromID, e.ToID} {
					if !seenNodes[string(nid)] {
						n, err := g.GetNode(nid)
						if err == nil {
							seenNodes[string(nid)] = true
							nodes = append(nodes, nodeToJSON(n))
						}
					}
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"nodes": nodes,
		"edges": edges,
	})
}

func exploreByLabel(w http.ResponseWriter, g *graph.Graph, label string, limit int) {
	allNodes, err := g.NodesByLabel(label)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if len(allNodes) > limit {
		allNodes = allNodes[:limit]
	}

	seenNodes := make(map[string]bool)
	seenEdges := make(map[string]bool)
	var nodes []any
	var edges []any

	for _, n := range allNodes {
		seenNodes[string(n.ID)] = true
		nodes = append(nodes, nodeToJSON(n))
	}

	// Get edges between these nodes
	for _, n := range allNodes {
		nEdges, err := g.Neighbors(n.ID)
		if err != nil {
			continue
		}
		for _, e := range nEdges {
			if !seenEdges[string(e.ID)] {
				seenEdges[string(e.ID)] = true
				edges = append(edges, edgeToJSON(e))
				// Include neighbor nodes if not already present
				for _, nid := range []graph.NodeID{e.FromID, e.ToID} {
					if !seenNodes[string(nid)] && len(nodes) < limit*2 {
						neighbor, err := g.GetNode(nid)
						if err == nil {
							seenNodes[string(nid)] = true
							nodes = append(nodes, nodeToJSON(neighbor))
						}
					}
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"nodes": nodes,
		"edges": edges,
	})
}

func handleMgmtGraphSearch(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		query := q.Get("q")
		label := q.Get("label")
		limitStr := q.Get("limit")

		if query == "" {
			writeError(w, http.StatusBadRequest, "q parameter required")
			return
		}

		limit := 20
		if limitStr != "" {
			if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 100 {
				limit = v
			}
		}

		queryLower := strings.ToLower(query)
		labels := knownLabels
		if label != "" {
			labels = []string{label}
		}

		seenNodes := make(map[string]bool)
		seenEdges := make(map[string]bool)
		var nodes []any
		var edges []any

		for _, lbl := range labels {
			if len(nodes) >= limit {
				break
			}
			allNodes, err := g.NodesByLabel(lbl)
			if err != nil {
				continue
			}
			for _, n := range allNodes {
				if len(nodes) >= limit {
					break
				}
				if matchesSearch(n, queryLower) && !seenNodes[string(n.ID)] {
					seenNodes[string(n.ID)] = true
					nodes = append(nodes, nodeToJSON(n))
				}
			}
		}

		// Collect edges for found nodes
		for _, n := range nodes {
			nm := n.(map[string]any)
			nid := nm["id"].(string)
			nEdges, err := g.Neighbors(graph.NodeID(nid))
			if err != nil {
				continue
			}
			for _, e := range nEdges {
				if !seenEdges[string(e.ID)] && seenNodes[string(e.FromID)] && seenNodes[string(e.ToID)] {
					seenEdges[string(e.ID)] = true
					edges = append(edges, edgeToJSON(e))
				}
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"nodes": nodes,
			"edges": edges,
		})
	}
}

// --- Config & Cluster ---

func handleMgmtConfig(opts *serverOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if opts.cfg == nil {
			writeError(w, http.StatusNotFound, "config not available")
			return
		}
		writeJSON(w, http.StatusOK, sanitizeConfig(opts.cfg))
	}
}

// configSettingSource describes a single config property with its source.
type configSettingSource struct {
	Key     string `json:"key"`
	Value   any    `json:"value"`
	Default any    `json:"default"`
	Source  string `json:"source"` // "default", "file", "env"
	EnvVar  string `json:"envVar"` // e.g. KAFGRAPH_HOST
}

func handleMgmtConfigDetailed(opts *serverOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if opts.cfg == nil {
			writeError(w, http.StatusNotFound, "config not available")
			return
		}

		// Define all settings grouped by section.
		type sectionDef struct {
			Name     string
			Settings []struct {
				Key      string
				Default  any
				GetValue func(*config.Config) any
			}
		}

		// Build the detailed config by section.
		sections := buildConfigSections(opts.cfg)
		writeJSON(w, http.StatusOK, sections)
	}
}

// buildConfigSections returns config settings grouped by section with source detection.
func buildConfigSections(cfg *config.Config) map[string][]configSettingSource {
	type entry struct {
		key      string
		envKey   string
		value    any
		defValue any
	}

	sections := map[string][]entry{
		"Server": {
			{"host", "KAFGRAPH_HOST", cfg.Host, "0.0.0.0"},
			{"port", "KAFGRAPH_PORT", cfg.Port, 7474},
			{"bolt_port", "KAFGRAPH_BOLT_PORT", cfg.BoltPort, 7687},
			{"log_level", "KAFGRAPH_LOG_LEVEL", cfg.LogLevel, "info"},
			{"log_format", "KAFGRAPH_LOG_FORMAT", cfg.LogFormat, "json"},
		},
		"Storage": {
			{"storage_engine", "KAFGRAPH_STORAGE_ENGINE", cfg.StorageEngine, "badger"},
			{"data_dir", "KAFGRAPH_DATA_DIR", cfg.DataDir, "./data"},
		},
		"Kafka": {
			{"kafka.brokers", "KAFGRAPH_KAFKA_BROKERS", cfg.Kafka.Brokers, "localhost:9092"},
			{"kafka.group_id", "KAFGRAPH_KAFKA_GROUP_ID", cfg.Kafka.GroupID, "kafgraph"},
			{"kafka.topic_prefix", "KAFGRAPH_KAFKA_TOPIC_PREFIX", cfg.Kafka.TopicPrefix, "group"},
		},
		"S3 / MinIO": {
			{"s3.endpoint", "KAFGRAPH_S3_ENDPOINT", cfg.S3.Endpoint, "localhost:9000"},
			{"s3.access_key", "KAFGRAPH_S3_ACCESS_KEY", "***REDACTED***", ""},
			{"s3.secret_key", "KAFGRAPH_S3_SECRET_KEY", "***REDACTED***", ""},
			{"s3.bucket", "KAFGRAPH_S3_BUCKET", cfg.S3.Bucket, "kafgraph"},
			{"s3.use_ssl", "KAFGRAPH_S3_USE_SSL", cfg.S3.UseSSL, false},
		},
		"Ingest": {
			{"ingest.enabled", "KAFGRAPH_INGEST_ENABLED", cfg.Ingest.Enabled, false},
			{"ingest.poll_interval", "KAFGRAPH_INGEST_POLL_INTERVAL", cfg.Ingest.PollInterval, "5s"},
			{"ingest.batch_size", "KAFGRAPH_INGEST_BATCH_SIZE", cfg.Ingest.BatchSize, 1000},
			{"ingest.namespace", "KAFGRAPH_INGEST_NAMESPACE", cfg.Ingest.Namespace, "kafgraph-ingest"},
		},
		"Reflection": {
			{"reflect.enabled", "KAFGRAPH_REFLECT_ENABLED", cfg.Reflect.Enabled, false},
			{"reflect.check_interval", "KAFGRAPH_REFLECT_CHECK_INTERVAL", cfg.Reflect.CheckInterval, "1m"},
			{"reflect.daily_time", "KAFGRAPH_REFLECT_DAILY_TIME", cfg.Reflect.DailyTime, "02:00"},
			{"reflect.weekly_day", "KAFGRAPH_REFLECT_WEEKLY_DAY", cfg.Reflect.WeeklyDay, "Monday"},
			{"reflect.weekly_time", "KAFGRAPH_REFLECT_WEEKLY_TIME", cfg.Reflect.WeeklyTime, "03:00"},
			{"reflect.monthly_day", "KAFGRAPH_REFLECT_MONTHLY_DAY", cfg.Reflect.MonthlyDay, 1},
			{"reflect.monthly_time", "KAFGRAPH_REFLECT_MONTHLY_TIME", cfg.Reflect.MonthlyTime, "04:00"},
			{"reflect.feedback_grace_period", "KAFGRAPH_REFLECT_FEEDBACK_GRACE_PERIOD", cfg.Reflect.FeedbackGracePeriod, "24h"},
			{"reflect.feedback_request_topic", "KAFGRAPH_REFLECT_FEEDBACK_REQUEST_TOPIC", cfg.Reflect.FeedbackRequestTopic, "kafgraph.feedback.requests"},
			{"reflect.feedback_top_n", "KAFGRAPH_REFLECT_FEEDBACK_TOP_N", cfg.Reflect.FeedbackTopN, 5},
		},
		"Cluster": {
			{"cluster.enabled", "KAFGRAPH_CLUSTER_ENABLED", cfg.Cluster.Enabled, false},
			{"cluster.node_name", "KAFGRAPH_CLUSTER_NODE_NAME", cfg.Cluster.NodeName, ""},
			{"cluster.bind_addr", "KAFGRAPH_CLUSTER_BIND_ADDR", cfg.Cluster.BindAddr, "0.0.0.0"},
			{"cluster.gossip_port", "KAFGRAPH_CLUSTER_GOSSIP_PORT", cfg.Cluster.GossipPort, 7946},
			{"cluster.rpc_port", "KAFGRAPH_CLUSTER_RPC_PORT", cfg.Cluster.RPCPort, 7948},
			{"cluster.num_partitions", "KAFGRAPH_CLUSTER_NUM_PARTITIONS", cfg.Cluster.NumPartitions, 16},
		},
	}

	result := make(map[string][]configSettingSource, len(sections))
	for section, entries := range sections {
		var settings []configSettingSource
		for _, e := range entries {
			source := detectSource(e.envKey, e.value, e.defValue)
			settings = append(settings, configSettingSource{
				Key:     e.key,
				Value:   e.value,
				Default: e.defValue,
				Source:  source,
				EnvVar:  e.envKey,
			})
		}
		result[section] = settings
	}
	return result
}

// detectSource determines if a config value came from env, file, or default.
func detectSource(envKey string, value, defValue any) string {
	// Check if env var is set.
	if _, ok := os.LookupEnv(envKey); ok {
		return "env"
	}
	// If value differs from default, it came from the config file.
	if fmt.Sprint(value) != fmt.Sprint(defValue) {
		return "file"
	}
	return "default"
}

func handleMgmtCluster(opts *serverOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if opts.cfg == nil || !opts.cfg.Cluster.Enabled || opts.membership == nil {
			// Single-node mode
			writeJSON(w, http.StatusOK, map[string]any{
				"enabled":    false,
				"members":    []any{},
				"partitions": map[string]any{},
				"self":       nil,
			})
			return
		}

		members := opts.membership.Members()
		self := opts.membership.Self()

		partitions := map[string]string{}
		if opts.partMap != nil {
			owners := opts.partMap.Owners()
			for p, owner := range owners {
				partitions[fmt.Sprintf("%d", p)] = owner.Name
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"enabled":    true,
			"self":       self,
			"members":    members,
			"partitions": partitions,
		})
	}
}

// --- Reflection ---

func handleMgmtReflectSummary(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		cycles, _ := g.NodesByLabel("ReflectionCycle")
		signals, _ := g.NodesByLabel("LearningSignal")

		byStatus := make(map[string]int)
		byType := make(map[string]int)
		feedbackPipeline := make(map[string]int)
		var totalImpact, totalRelevance, totalValue float64
		scoredCount := 0

		for _, c := range cycles {
			status, _ := c.Properties["status"].(string)
			if status != "" {
				byStatus[status]++
			}
			cycleType, _ := c.Properties["type"].(string)
			if cycleType != "" {
				byType[cycleType]++
			}
			fbStatus, _ := c.Properties["humanFeedbackStatus"].(string)
			if fbStatus != "" {
				feedbackPipeline[fbStatus]++
			}
		}

		for _, s := range signals {
			if impact, ok := toFloat64(s.Properties["impact"]); ok {
				totalImpact += impact
				scoredCount++
			}
			if rel, ok := toFloat64(s.Properties["relevance"]); ok {
				totalRelevance += rel
			}
			if val, ok := toFloat64(s.Properties["valueContribution"]); ok {
				totalValue += val
			}
		}

		avgScores := map[string]float64{
			"impact":            0,
			"relevance":         0,
			"valueContribution": 0,
		}
		if scoredCount > 0 {
			avgScores["impact"] = totalImpact / float64(scoredCount)
			avgScores["relevance"] = totalRelevance / float64(scoredCount)
			avgScores["valueContribution"] = totalValue / float64(scoredCount)
		}

		// Compute last-run timestamp (global and per-agent).
		var lastRunGlobal string
		lastRunByAgent := make(map[string]string)
		for _, c := range cycles {
			completedAt, _ := c.Properties["completedAt"].(string)
			if completedAt == "" {
				continue
			}
			if completedAt > lastRunGlobal {
				lastRunGlobal = completedAt
			}
			agentID, _ := c.Properties["agentId"].(string)
			if agentID != "" && completedAt > lastRunByAgent[agentID] {
				lastRunByAgent[agentID] = completedAt
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"totalCycles":      len(cycles),
			"byStatus":         byStatus,
			"byType":           byType,
			"feedbackPipeline": feedbackPipeline,
			"totalSignals":     len(signals),
			"averageScores":    avgScores,
			"lastRun":          lastRunGlobal,
			"lastRunByAgent":   lastRunByAgent,
		})
	}
}

func handleMgmtReflectCycles(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		limitStr := q.Get("limit")
		offsetStr := q.Get("offset")
		typeFilter := q.Get("type")
		statusFilter := q.Get("status")

		limit := 20
		if limitStr != "" {
			if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 100 {
				limit = v
			}
		}
		offset := 0
		if offsetStr != "" {
			if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
				offset = v
			}
		}

		allCycles, _ := g.NodesByLabel("ReflectionCycle")

		// Filter
		var filtered []*graph.Node
		for _, c := range allCycles {
			if typeFilter != "" {
				t, _ := c.Properties["type"].(string)
				if t != typeFilter {
					continue
				}
			}
			if statusFilter != "" {
				s, _ := c.Properties["status"].(string)
				if s != statusFilter {
					continue
				}
			}
			filtered = append(filtered, c)
		}

		// Sort by creation time (newest first)
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
		})

		// Paginate
		total := len(filtered)
		if offset > total {
			offset = total
		}
		end := offset + limit
		if end > total {
			end = total
		}
		page := filtered[offset:end]

		// Get top signals for each cycle
		var cycles []any
		for _, c := range page {
			entry := nodeToJSON(c)
			// Attempt to get associated signals
			cEdges, err := g.Neighbors(c.ID)
			if err == nil {
				var topSignals []map[string]any
				for _, e := range cEdges {
					if e.Label == "HAS_SIGNAL" {
						sig, err := g.GetNode(e.ToID)
						if err == nil && len(topSignals) < 3 {
							topSignals = append(topSignals, map[string]any{
								"id":    sig.ID,
								"label": sig.Label,
								"type":  sig.Properties["type"],
							})
						}
					}
				}
				entry["topSignals"] = topSignals
			}
			cycles = append(cycles, entry)
		}

		if cycles == nil {
			cycles = []any{}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"cycles": cycles,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

// --- Reflection Trigger ---

type reflectTriggerRequest struct {
	AgentID     string `json:"agentId"`
	WindowHours int    `json:"windowHours"`
}

func handleMgmtReflectTrigger(g *graph.Graph, opts *serverOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if opts.brain == nil {
			writeError(w, http.StatusNotImplemented, "brain service not available")
			return
		}

		var req reflectTriggerRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}

		// Default to all agents if no agentId specified.
		agentIDs := []string{req.AgentID}
		if req.AgentID == "" {
			agents, err := g.NodesByLabel("Agent")
			if err != nil || len(agents) == 0 {
				writeError(w, http.StatusBadRequest, "no agents found; specify agentId")
				return
			}
			agentIDs = make([]string, 0, len(agents))
			for _, a := range agents {
				name, _ := a.Properties["name"].(string)
				if name == "" {
					name = string(a.ID)
				}
				agentIDs = append(agentIDs, name)
			}
		}

		windowHours := req.WindowHours
		if windowHours <= 0 {
			windowHours = 24
		}

		var results []map[string]any
		for _, agentID := range agentIDs {
			out, err := opts.brain.Reflect(brain.ReflectInput{
				AgentID:     agentID,
				WindowHours: windowHours,
			})
			if err != nil {
				results = append(results, map[string]any{
					"agentId": agentID,
					"error":   err.Error(),
				})
				continue
			}
			results = append(results, map[string]any{
				"agentId": agentID,
				"cycleId": out.CycleID,
				"signals": len(out.LearningSignals),
				"summary": out.Summary,
			})
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"triggered": len(results),
			"results":   results,
		})
	}
}

// --- Activity ---

func handleMgmtActivity(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		hoursStr := q.Get("hours")
		limitStr := q.Get("limit")

		hours := 24
		if hoursStr != "" {
			if v, err := strconv.Atoi(hoursStr); err == nil && v > 0 && v <= 168 {
				hours = v
			}
		}
		limit := 100
		if limitStr != "" {
			if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 500 {
				limit = v
			}
		}

		cutoff := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
		var events []any

		for _, label := range knownLabels {
			nodes, err := g.NodesByLabel(label)
			if err != nil {
				continue
			}
			for _, n := range nodes {
				if n.CreatedAt.After(cutoff) {
					events = append(events, nodeToJSON(n))
				}
			}
		}

		// Sort by creation time (newest first)
		sort.Slice(events, func(i, j int) bool {
			ti := events[i].(map[string]any)["createdAt"].(string)
			tj := events[j].(map[string]any)["createdAt"].(string)
			return ti > tj
		})

		if len(events) > limit {
			events = events[:limit]
		}

		if events == nil {
			events = []any{}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"events": events,
		})
	}
}

// --- Skills by Agent ---

func handleMgmtSkillsByAgent(g *graph.Graph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		window := r.URL.Query().Get("window")
		if window == "" {
			window = "day"
		}
		agentFilter := r.URL.Query().Get("agent") // optional per-agent filter

		agents, _ := g.NodesByLabel("Agent")

		type timelineBucket struct {
			Time  string `json:"time"`
			Count int    `json:"count"`
		}
		type skillUsage struct {
			Skill    string           `json:"skill"`
			Total    int              `json:"totalUses"`
			Timeline []timelineBucket `json:"timeline"`
		}
		type skillHistoryEvent struct {
			Skill         string `json:"skill"`
			DeclaredAt    string `json:"declaredAt,omitempty"`
			RemovedAt     string `json:"removedAt,omitempty"`
			RosterVersion int    `json:"rosterVersion,omitempty"`
			Active        bool   `json:"active"`
		}
		type agentEntry struct {
			ID             string              `json:"id"`
			Name           string              `json:"name"`
			DeclaredSkills []string            `json:"declaredSkills"`
			SkillUsage     []skillUsage        `json:"skillUsage"`
			SkillHistory   []skillHistoryEvent `json:"skillHistory"`
		}

		// Global skill roster: skill -> set of agent names
		rosterMap := make(map[string]map[string]bool)
		rosterUsage := make(map[string]int)

		var agentEntries []agentEntry

		for _, agent := range agents {
			name, _ := agent.Properties["name"].(string)
			if name == "" {
				name, _ = agent.Properties["agentName"].(string)
			}
			if name == "" {
				// Extract from ID: "n:Agent:researcher" -> "researcher"
				parts := strings.SplitN(string(agent.ID), ":", 3)
				if len(parts) == 3 {
					name = parts[2]
				} else {
					name = string(agent.ID)
				}
			}

			// Apply per-agent filter
			if agentFilter != "" && !strings.EqualFold(name, agentFilter) {
				continue
			}

			edges, err := g.Neighbors(agent.ID)
			if err != nil {
				edges = nil
			}

			declared := make(map[string]bool)
			var msgIDs []graph.NodeID
			var history []skillHistoryEvent

			for _, e := range edges {
				if e.Label == "HAS_SKILL" && e.FromID == agent.ID {
					skillNode, err := g.GetNode(e.ToID)
					if err != nil {
						continue
					}
					sn, _ := skillNode.Properties["skillName"].(string)
					if sn == "" {
						continue
					}

					// Build history event from edge properties
					evt := skillHistoryEvent{Skill: sn, Active: true}
					if da, ok := e.Properties["declaredAt"].(string); ok {
						evt.DeclaredAt = da
					}
					if rv, ok := toInt(e.Properties["rosterVersion"]); ok {
						evt.RosterVersion = rv
					}
					if ra, ok := e.Properties["removedAt"].(string); ok {
						evt.RemovedAt = ra
						evt.Active = false
					}
					history = append(history, evt)

					// Only count as active/declared if not removed
					if evt.Active {
						declared[sn] = true
						if rosterMap[sn] == nil {
							rosterMap[sn] = make(map[string]bool)
						}
						rosterMap[sn][name] = true
					}
				}
				if e.Label == "AUTHORED" && e.FromID == agent.ID {
					msgIDs = append(msgIDs, e.ToID)
				}
			}

			// Sort history by declaredAt, then skill name
			sort.Slice(history, func(i, j int) bool {
				if history[i].DeclaredAt != history[j].DeclaredAt {
					return history[i].DeclaredAt < history[j].DeclaredAt
				}
				return history[i].Skill < history[j].Skill
			})

			// For each authored message, check USES_SKILL edges
			usageBuckets := make(map[string]map[string]int) // skill -> bucket -> count
			usageTotals := make(map[string]int)

			for _, msgID := range msgIDs {
				msg, err := g.GetNode(msgID)
				if err != nil {
					continue
				}
				msgEdges, err := g.Neighbors(msgID)
				if err != nil {
					continue
				}
				for _, me := range msgEdges {
					if me.Label == "USES_SKILL" && me.FromID == msgID {
						skillNode, err := g.GetNode(me.ToID)
						if err != nil {
							continue
						}
						sn, _ := skillNode.Properties["skillName"].(string)
						if sn == "" {
							continue
						}
						usageTotals[sn]++
						rosterUsage[sn]++

						bucket := timeBucket(msg.CreatedAt, window)
						if usageBuckets[sn] == nil {
							usageBuckets[sn] = make(map[string]int)
						}
						usageBuckets[sn][bucket]++

						if rosterMap[sn] == nil {
							rosterMap[sn] = make(map[string]bool)
						}
						rosterMap[sn][name] = true
					}
				}
			}

			declaredList := make([]string, 0, len(declared))
			for s := range declared {
				declaredList = append(declaredList, s)
			}
			sort.Strings(declaredList)

			var usageList []skillUsage
			for sn, total := range usageTotals {
				buckets := usageBuckets[sn]
				sortedKeys := make([]string, 0, len(buckets))
				for k := range buckets {
					sortedKeys = append(sortedKeys, k)
				}
				sort.Strings(sortedKeys)

				var timeline []timelineBucket
				for _, k := range sortedKeys {
					timeline = append(timeline, timelineBucket{Time: k, Count: buckets[k]})
				}
				usageList = append(usageList, skillUsage{
					Skill:    sn,
					Total:    total,
					Timeline: timeline,
				})
			}
			sort.Slice(usageList, func(i, j int) bool {
				return usageList[i].Skill < usageList[j].Skill
			})

			if history == nil {
				history = []skillHistoryEvent{}
			}

			agentEntries = append(agentEntries, agentEntry{
				ID:             string(agent.ID),
				Name:           name,
				DeclaredSkills: declaredList,
				SkillUsage:     usageList,
				SkillHistory:   history,
			})
		}

		sort.Slice(agentEntries, func(i, j int) bool {
			return agentEntries[i].Name < agentEntries[j].Name
		})

		// Build skill roster
		type rosterEntry struct {
			Skill     string   `json:"skill"`
			Agents    []string `json:"agents"`
			TotalUses int      `json:"totalUses"`
		}
		var roster []rosterEntry
		for skill, agentSet := range rosterMap {
			agentNames := make([]string, 0, len(agentSet))
			for a := range agentSet {
				agentNames = append(agentNames, a)
			}
			sort.Strings(agentNames)
			roster = append(roster, rosterEntry{
				Skill:     skill,
				Agents:    agentNames,
				TotalUses: rosterUsage[skill],
			})
		}
		sort.Slice(roster, func(i, j int) bool {
			return roster[i].Skill < roster[j].Skill
		})

		if agentEntries == nil {
			agentEntries = []agentEntry{}
		}
		if roster == nil {
			roster = []rosterEntry{}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"agents":      agentEntries,
			"skillRoster": roster,
		})
	}
}

// timeBucket truncates a timestamp to the given window granularity.
func timeBucket(t time.Time, window string) string {
	switch window {
	case "hour":
		return t.UTC().Truncate(time.Hour).Format(time.RFC3339)
	case "week":
		// Truncate to Monday of the week.
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := t.UTC().AddDate(0, 0, -(weekday - 1))
		return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	default: // "day"
		return t.UTC().Truncate(24 * time.Hour).Format(time.RFC3339)
	}
}

// --- Helpers ---

func nodeToJSON(n *graph.Node) map[string]any {
	return map[string]any{
		"id":         string(n.ID),
		"label":      n.Label,
		"properties": n.Properties,
		"createdAt":  n.CreatedAt.Format(time.RFC3339),
	}
}

func edgeToJSON(e *graph.Edge) map[string]any {
	return map[string]any{
		"id":         string(e.ID),
		"label":      e.Label,
		"fromId":     string(e.FromID),
		"toId":       string(e.ToID),
		"properties": e.Properties,
		"createdAt":  e.CreatedAt.Format(time.RFC3339),
	}
}

func countByLabel(g *graph.Graph, label string) int {
	// Use IndexedStorage if available for efficient counting
	backend := g.StorageBackend()
	if is, ok := backend.(graph.IndexedStorage); ok {
		ids, err := is.NodeIDsByLabel(label)
		if err == nil {
			return len(ids)
		}
	}
	// Fallback: load all nodes
	nodes, err := g.NodesByLabel(label)
	if err != nil {
		return 0
	}
	return len(nodes)
}

func countEdgesByLabel(g *graph.Graph, label string) int {
	backend := g.StorageBackend()
	if is, ok := backend.(graph.IndexedStorage); ok {
		ids, err := is.EdgeIDsByLabel(label)
		if err == nil {
			return len(ids)
		}
	}
	return 0
}

func matchesSearch(n *graph.Node, queryLower string) bool {
	if strings.Contains(strings.ToLower(string(n.ID)), queryLower) {
		return true
	}
	if strings.Contains(strings.ToLower(n.Label), queryLower) {
		return true
	}
	for _, v := range n.Properties {
		if s, ok := v.(string); ok {
			if strings.Contains(strings.ToLower(s), queryLower) {
				return true
			}
		}
	}
	return false
}

func sanitizeConfig(cfg *config.Config) map[string]any {
	// Build a clean map with snake_case keys matching mapstructure tags.
	result := map[string]any{
		"host":           cfg.Host,
		"port":           cfg.Port,
		"bolt_port":      cfg.BoltPort,
		"data_dir":       cfg.DataDir,
		"storage_engine": cfg.StorageEngine,
		"log_level":      cfg.LogLevel,
		"log_format":     cfg.LogFormat,
		"kafka": map[string]any{
			"brokers":      cfg.Kafka.Brokers,
			"group_id":     cfg.Kafka.GroupID,
			"topic_prefix": cfg.Kafka.TopicPrefix,
		},
		"s3": map[string]any{
			"endpoint":   cfg.S3.Endpoint,
			"access_key": "***REDACTED***",
			"secret_key": "***REDACTED***",
			"bucket":     cfg.S3.Bucket,
			"use_ssl":    cfg.S3.UseSSL,
		},
		"ingest": map[string]any{
			"enabled":       cfg.Ingest.Enabled,
			"poll_interval": cfg.Ingest.PollInterval,
			"batch_size":    cfg.Ingest.BatchSize,
			"namespace":     cfg.Ingest.Namespace,
		},
		"reflect": map[string]any{
			"enabled":               cfg.Reflect.Enabled,
			"check_interval":        cfg.Reflect.CheckInterval,
			"daily_time":            cfg.Reflect.DailyTime,
			"weekly_day":            cfg.Reflect.WeeklyDay,
			"weekly_time":           cfg.Reflect.WeeklyTime,
			"monthly_day":           cfg.Reflect.MonthlyDay,
			"monthly_time":          cfg.Reflect.MonthlyTime,
			"feedback_grace_period": cfg.Reflect.FeedbackGracePeriod,
		},
		"cluster": map[string]any{
			"enabled":        cfg.Cluster.Enabled,
			"node_name":      cfg.Cluster.NodeName,
			"bind_addr":      cfg.Cluster.BindAddr,
			"gossip_port":    cfg.Cluster.GossipPort,
			"rpc_port":       cfg.Cluster.RPCPort,
			"seeds":          cfg.Cluster.Seeds,
			"num_partitions": cfg.Cluster.NumPartitions,
		},
	}

	return result
}

func toInt(v any) (int, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case float64:
		return int(t), true
	case json.Number:
		i, err := t.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}

func toFloat64(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
