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

// Package demo provides pre-built scenarios for populating KafGraph
// with realistic agent conversation data without a live KafScale broker.
package demo

import (
	"encoding/json"
	"time"

	"github.com/scalytics/kafgraph/internal/ingest"
)

const (
	// Topic is the KafScale topic for the blog-team scenario.
	Topic = "group.blog-team"
	// Partition is the default partition.
	Partition int32 = 0
	// CorrelationID ties all envelopes to the same conversation.
	CorrelationID = "blog-draft-2026"
)

// Agent IDs.
const (
	Coordinator = "coordinator"
	Researcher  = "researcher"
	Editor      = "editor"
	Formatter   = "formatter"
)

// BlogTeamScenario returns 47 ingest.Records representing a multi-agent
// blog-writing pipeline. The records exercise every envelope type:
// announce, roster, orchestrator, request, response, task_status,
// skill_request, skill_response, memory, and audit.
// Three agents demonstrate REQ-009 roster evolution:
//   - Researcher starts with web_search + summarize, gains deep_search
//   - Editor starts with rewrite + tone_check, gains citation_check
//   - Formatter starts with ascii_doc + proofread, gains format_html
func BlogTeamScenario() []ingest.Record {
	t0 := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)

	return []ingest.Record{
		// ── Agent announcements ──────────────────────────────────
		env(0, t0, ingest.TypeAnnounce, Coordinator, ingest.AnnouncePayload{
			AgentID: Coordinator, AgentName: "Coordinator", Action: "join", GroupName: "blog-team",
		}),
		env(1, t0.Add(1*time.Second), ingest.TypeAnnounce, Researcher, ingest.AnnouncePayload{
			AgentID: Researcher, AgentName: "Researcher", Action: "join", GroupName: "blog-team",
		}),
		env(2, t0.Add(2*time.Second), ingest.TypeAnnounce, Editor, ingest.AnnouncePayload{
			AgentID: Editor, AgentName: "Editor", Action: "join", GroupName: "blog-team",
		}),
		env(3, t0.Add(3*time.Second), ingest.TypeAnnounce, Formatter, ingest.AnnouncePayload{
			AgentID: Formatter, AgentName: "Formatter", Action: "join", GroupName: "blog-team",
		}),

		// ── Skill rosters (Version 1 — initial declarations) ────
		env(4, t0.Add(4*time.Second), ingest.TypeRoster, Researcher, ingest.RosterPayload{
			AgentID: Researcher, Skills: []string{"web_search", "summarize"},
			Version: 1, Action: "full",
		}),
		env(5, t0.Add(5*time.Second), ingest.TypeRoster, Editor, ingest.RosterPayload{
			AgentID: Editor, Skills: []string{"rewrite", "tone_check"},
			Version: 1, Action: "full",
		}),
		env(6, t0.Add(6*time.Second), ingest.TypeRoster, Formatter, ingest.RosterPayload{
			AgentID: Formatter, Skills: []string{"ascii_doc", "proofread"},
			Version: 1, Action: "full",
		}),

		// ── Phase 1: Research ────────────────────────────────────
		env(7, t0.Add(10*time.Second), ingest.TypeOrchestrator, Coordinator, ingest.OrchestratorPayload{
			Action: "delegate", FromAgent: Coordinator, ToAgent: Researcher, TaskID: "research-blog",
		}),
		env(8, t0.Add(11*time.Second), ingest.TypeRequest, Coordinator, ingest.TaskRequestPayload{
			TaskID: "research-blog", RequestText: "Please research and enrich this blog draft about distributed agent systems", TargetAgent: Researcher,
		}),
		env(9, t0.Add(15*time.Second), ingest.TypeTaskStatus, Researcher, ingest.TaskStatusPayload{
			TaskID: "research-blog", Status: "in_progress",
		}),
		env(10, t0.Add(20*time.Second), ingest.TypeSkillRequest, Researcher, ingest.SkillRequestPayload{
			SkillName: "web_search", Parameters: rawJSON(map[string]string{"query": "distributed agent architectures 2026"}),
		}),
		env(11, t0.Add(25*time.Second), ingest.TypeSkillResponse, Researcher, ingest.SkillResponsePayload{
			SkillName: "web_search", Result: "Found 12 relevant papers on multi-agent coordination, consensus protocols, and shared-memory architectures. Key themes: eventual consistency, agent-local caching, gossip-based discovery.", InReplyTo: "offset-10",
		}),
		env(12, t0.Add(30*time.Second), ingest.TypeSkillRequest, Researcher, ingest.SkillRequestPayload{
			SkillName: "summarize", Parameters: rawJSON(map[string]string{"text": "Condense the web search findings into key points for the blog draft"}),
		}),
		env(13, t0.Add(35*time.Second), ingest.TypeSkillResponse, Researcher, ingest.SkillResponsePayload{
			SkillName: "summarize", Result: "Key points: (1) Agents benefit from shared graph memory for context retention. (2) Gossip protocols enable peer discovery without central registry. (3) Reflection cycles improve decision quality over time.", InReplyTo: "offset-12",
		}),
		env(14, t0.Add(40*time.Second), ingest.TypeMemory, Researcher, ingest.MemoryPayload{
			Key: "research-findings", Value: "Distributed agent systems benefit from: shared graph memory, gossip-based discovery, and periodic reflection cycles. Citations: AgentNet 2026, MultiMind Survey.", References: []string{Coordinator, Editor},
		}),
		env(15, t0.Add(45*time.Second), ingest.TypeResponse, Researcher, ingest.TaskResponsePayload{
			TaskID: "research-blog", ResponseText: "Here's the enriched draft with research citations on distributed agent architectures, shared memory patterns, and reflection cycles.", InReplyTo: "offset-8",
		}),
		env(16, t0.Add(50*time.Second), ingest.TypeTaskStatus, Researcher, ingest.TaskStatusPayload{
			TaskID: "research-blog", Status: "completed",
		}),
		env(17, t0.Add(51*time.Second), ingest.TypeAudit, Researcher, ingest.AuditPayload{
			Action: "task_completed", Resource: "blog-draft", Outcome: "success",
		}),

		// ── Researcher gains deep_search skill (REQ-009 roster evolution) ──
		env(18, t0.Add(55*time.Second), ingest.TypeRoster, Researcher, ingest.RosterPayload{
			AgentID: Researcher, Skills: []string{"deep_search"},
			Version: 2, Action: "add",
		}),

		// ── Phase 2: Editing ─────────────────────────────────────
		env(19, t0.Add(60*time.Second), ingest.TypeOrchestrator, Coordinator, ingest.OrchestratorPayload{
			Action: "delegate", FromAgent: Coordinator, ToAgent: Editor, TaskID: "edit-blog",
		}),
		env(20, t0.Add(61*time.Second), ingest.TypeRequest, Coordinator, ingest.TaskRequestPayload{
			TaskID: "edit-blog", RequestText: "Please sharpen this enriched draft for clarity and tone, focusing on technical accuracy and readability", TargetAgent: Editor,
		}),
		env(21, t0.Add(65*time.Second), ingest.TypeTaskStatus, Editor, ingest.TaskStatusPayload{
			TaskID: "edit-blog", Status: "in_progress",
		}),
		env(22, t0.Add(70*time.Second), ingest.TypeSkillRequest, Editor, ingest.SkillRequestPayload{
			SkillName: "tone_check", Parameters: rawJSON(map[string]string{"text": "Analyze the tone of the enriched blog draft for consistency"}),
		}),
		env(23, t0.Add(75*time.Second), ingest.TypeSkillResponse, Editor, ingest.SkillResponsePayload{
			SkillName: "tone_check", Result: "Tone analysis: mostly technical/neutral. Sections 2-3 shift to informal. Recommend aligning to professional-technical throughout.", InReplyTo: "offset-22",
		}),
		env(24, t0.Add(80*time.Second), ingest.TypeSkillRequest, Editor, ingest.SkillRequestPayload{
			SkillName: "rewrite", Parameters: rawJSON(map[string]string{"sections": "2,3", "style": "professional-technical"}),
		}),
		env(25, t0.Add(85*time.Second), ingest.TypeSkillResponse, Editor, ingest.SkillResponsePayload{
			SkillName: "rewrite", Result: "Rewritten sections 2-3 with consistent professional-technical tone. Improved transitions between research findings and practical implications.", InReplyTo: "offset-24",
		}),
		env(26, t0.Add(90*time.Second), ingest.TypeMemory, Editor, ingest.MemoryPayload{
			Key: "editorial-notes", Value: "Style decisions: professional-technical tone throughout, active voice preferred, acronyms spelled out on first use. Rewrote sections 2-3 for consistency.", References: []string{Coordinator, Formatter},
		}),
		env(27, t0.Add(95*time.Second), ingest.TypeResponse, Editor, ingest.TaskResponsePayload{
			TaskID: "edit-blog", ResponseText: "Here's the sharpened version with improved flow, consistent tone, and better transitions between sections.", InReplyTo: "offset-20",
		}),
		env(28, t0.Add(100*time.Second), ingest.TypeTaskStatus, Editor, ingest.TaskStatusPayload{
			TaskID: "edit-blog", Status: "completed",
		}),
		env(29, t0.Add(101*time.Second), ingest.TypeAudit, Editor, ingest.AuditPayload{
			Action: "task_completed", Resource: "blog-draft-v2", Outcome: "success",
		}),

		// ── Editor gains citation_check skill (REQ-009 roster evolution) ──
		env(30, t0.Add(105*time.Second), ingest.TypeRoster, Editor, ingest.RosterPayload{
			AgentID: Editor, Skills: []string{"citation_check"},
			Version: 2, Action: "add",
		}),

		// ── Phase 3: Formatting ──────────────────────────────────
		env(31, t0.Add(110*time.Second), ingest.TypeOrchestrator, Coordinator, ingest.OrchestratorPayload{
			Action: "delegate", FromAgent: Coordinator, ToAgent: Formatter, TaskID: "format-blog",
		}),
		env(32, t0.Add(111*time.Second), ingest.TypeRequest, Coordinator, ingest.TaskRequestPayload{
			TaskID: "format-blog", RequestText: "Please format and finalize the edited blog post for publication", TargetAgent: Formatter,
		}),
		env(33, t0.Add(115*time.Second), ingest.TypeTaskStatus, Formatter, ingest.TaskStatusPayload{
			TaskID: "format-blog", Status: "in_progress",
		}),
		env(34, t0.Add(120*time.Second), ingest.TypeSkillRequest, Formatter, ingest.SkillRequestPayload{
			SkillName: "proofread", Parameters: rawJSON(map[string]string{"text": "Check the edited blog draft for grammar, spelling, and punctuation"}),
		}),
		env(35, t0.Add(125*time.Second), ingest.TypeSkillResponse, Formatter, ingest.SkillResponsePayload{
			SkillName: "proofread", Result: "Proofread complete: 3 minor typos fixed, 1 comma splice corrected, all citations verified.", InReplyTo: "offset-34",
		}),
		env(36, t0.Add(130*time.Second), ingest.TypeSkillRequest, Formatter, ingest.SkillRequestPayload{
			SkillName: "ascii_doc", Parameters: rawJSON(map[string]string{"template": "blog-publication", "toc": "true"}),
		}),
		env(37, t0.Add(135*time.Second), ingest.TypeSkillResponse, Formatter, ingest.SkillResponsePayload{
			SkillName: "ascii_doc", Result: "Applied AsciiDoc blog template: added front matter, table of contents, code block formatting, and cross-references.", InReplyTo: "offset-36",
		}),

		// ── Formatter gains format_html skill (REQ-009 roster evolution) ──
		env(38, t0.Add(138*time.Second), ingest.TypeRoster, Formatter, ingest.RosterPayload{
			AgentID: Formatter, Skills: []string{"format_html"},
			Version: 2, Action: "add",
		}),
		env(39, t0.Add(140*time.Second), ingest.TypeSkillRequest, Formatter, ingest.SkillRequestPayload{
			SkillName: "format_html", Parameters: rawJSON(map[string]string{"source": "ascii_doc", "responsive": "true"}),
		}),
		env(40, t0.Add(145*time.Second), ingest.TypeSkillResponse, Formatter, ingest.SkillResponsePayload{
			SkillName: "format_html", Result: "Converted AsciiDoc to responsive HTML: semantic markup, responsive image tags, and syntax-highlighted code blocks.", InReplyTo: "offset-39",
		}),

		env(41, t0.Add(150*time.Second), ingest.TypeMemory, Formatter, ingest.MemoryPayload{
			Key: "final-blog", Value: "Publication-ready blog post: 'Building Distributed Agent Systems with Shared Graph Memory'. 2,400 words, 3 code examples, 5 citations. Available in AsciiDoc and HTML.", References: []string{Coordinator, Researcher, Editor},
		}),
		env(42, t0.Add(155*time.Second), ingest.TypeResponse, Formatter, ingest.TaskResponsePayload{
			TaskID: "format-blog", ResponseText: "Blog is formatted in AsciiDoc and converted to HTML. Fixed typos, verified all citations, and generated responsive output.", InReplyTo: "offset-32",
		}),
		env(43, t0.Add(160*time.Second), ingest.TypeTaskStatus, Formatter, ingest.TaskStatusPayload{
			TaskID: "format-blog", Status: "completed",
		}),
		env(44, t0.Add(161*time.Second), ingest.TypeAudit, Formatter, ingest.AuditPayload{
			Action: "task_completed", Resource: "blog-final", Outcome: "success",
		}),

		// ── Pipeline completion ───────────────────────────────────
		env(45, t0.Add(170*time.Second), ingest.TypeOrchestrator, Coordinator, ingest.OrchestratorPayload{
			Action: "report", FromAgent: Coordinator, ToAgent: Coordinator, TaskID: "blog-pipeline",
		}),
		env(46, t0.Add(171*time.Second), ingest.TypeAudit, Coordinator, ingest.AuditPayload{
			Action: "pipeline_completed", Resource: "blog-post", Outcome: "success",
		}),
	}
}

// env builds a single ingest.Record from typed payload data.
func env(offset int64, ts time.Time, typ ingest.EnvelopeType, sender string, payload any) ingest.Record {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		panic("demo: marshal payload: " + err.Error())
	}

	envelope := ingest.GroupEnvelope{
		Type:          typ,
		CorrelationID: CorrelationID,
		SenderID:      sender,
		Timestamp:     ts,
		Payload:       payloadBytes,
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		panic("demo: marshal envelope: " + err.Error())
	}

	return ingest.Record{
		Source: ingest.SourceOffset{
			Topic:     Topic,
			Partition: Partition,
			Offset:    offset,
		},
		Value: data,
	}
}

// rawJSON marshals v to json.RawMessage.
func rawJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic("demo: marshal raw json: " + err.Error())
	}
	return data
}
