# KafGraph — Initial Idea

*Captured: 2026-03-02*

---

## Origin

This document preserves the original, unedited description of the KafGraph concept as
stated by its initiator. It is the authoritative record of intent and must not be
modified once committed.

---

## The Vision — in the Author's Own Words

> We need a specification for a graph database system, which runs in distributed mode,
> using Kafka Topics as the foundation of incoming data. The idea is to implement a
> reflection service for individual agents which reflect on their own work from the past,
> and also reflect over the contributions and activities of other agents in the past, on
> a regular basis. We aim on daily reflections, weekly reflections and monthly reflections
> in the first round. The idea is to find out what happened during activity of an agent
> team over time. We aim on tracking the conversation data in Kafka topics. The KafClaw
> project defines such Agent groups using shared topics for conversation and long-term
> audits, but here, we go beyond audits. We aim on learning from experiences. We do not
> know, what to learn on which path, but we aim on linking the conversations according to
> impact, relevance and value contribution in such a way, that the agents can improve
> their skills by "post-processing" their conversation and their human feedback. And in
> case no human feedback is available yet, they must explicitly request this from the
> owner.
>
> Please document this "initial description" in a folder SPEC/initial-idea.md and then
> create a requirements document capturing such a database, running per agent and also in
> collaboration mode as a distributed service. We aim on collocation of KafGraph with
> KafScale brokers as Processors, so that they can directly process the S3 data without
> overloading the brokers.
>
> Create a solution design. Do research on existing graph processing systems in Golang,
> and try to find a Neo4J compatible solution.

---

## Interpretation and Key Concepts

The following bullet points distil the core concepts from the description above without
changing any meaning. They serve as a quick-reference summary.

### 1. Distributed Graph Database on Kafka

- The graph database is **not a sidecar** — it is a first-class distributed system.
- Apache Kafka Topics are the **primary ingestion channel** for all incoming data. Every
  node, edge, and property update enters the system through a topic.
- The system must co-locate with **KafScale** broker-Processors so it can read directly
  from S3 (Kafka tiered storage or KafScale's own S3 layer) without routing all data
  back through the brokers.

### 2. Agent Reflection Service

- Every agent in the system has an **individual reflection loop**.
- Reflection is **temporal**: daily, weekly, and monthly cycles are the first target.
- Reflection is **bilateral**: an agent reflects on
  - its *own* past work and conversations, and
  - the *other* agents' contributions and activities.
- The goal is to understand *what happened* across an agent team over time — not just
  to audit, but to derive actionable learning signals.

### 3. Learning Without a Fixed Curriculum

- The system does **not prescribe what to learn** in advance.
- Instead, conversations are **linked by three qualitative dimensions**:
  - **Impact** — what observable effect did this exchange have on outcomes?
  - **Relevance** — how closely does it relate to the core task or goal?
  - **Value Contribution** — what measurable benefit did it add to the team or the
    product?
- These links form a continuously growing **knowledge graph** from which learning paths
  emerge organically.

### 4. Post-Processing Loop

- Agents re-examine their stored conversations and extract lessons.
- Human feedback, when available, is a **primary signal** that is attached to
  conversation nodes in the graph.
- When human feedback is **absent**, the agent is obligated to **request it explicitly**
  from the designated owner — it must not silently infer quality without human input.

### 5. Relationship to KafClaw

- **KafClaw** already defines agent groups with shared Kafka topics for real-time
  conversation and long-term audit trails.
- **KafGraph** builds on top of KafClaw's topic structure but elevates the stored data
  from passive audit records to **active, queryable, reflective knowledge**.
- KafGraph is therefore the *learning layer* that sits above KafClaw's *audit layer*.

### 6. Deployment Model

- **Per-agent mode**: each agent runs its own embedded graph instance for local
  introspection and fast self-reflection.
- **Collaborative / distributed mode**: all per-agent graphs federate into a shared
  cluster for cross-agent analysis and team-level reflection.
- The system must **not overload Kafka brokers** — S3 data is consumed directly at the
  KafScale Processor layer.

---

## Open Questions (as of initial capture)

| # | Question | Owner |
|---|----------|-------|
| Q1 | What specific Kafka topic schema does KafClaw use for conversations? | KafClaw team |
| Q2 | What is the expected conversation volume per agent per day? | Product |
| Q3 | How is "impact" measured — heuristically, via LLM scoring, or human rating? | Architecture |
| Q4 | What SLA is required for reflection results to be available after a cycle ends? | Product |
| Q5 | Is a Neo4j-compatible Cypher query surface required, or is it a preference? | Architecture |
| Q6 | Which S3 provider and KafScale version are in scope for the first release? | Infrastructure |
| Q7 | Who is "the owner" for human feedback requests — individual human, team lead, or configurable? | Product |

---

*This document is intentionally left as the raw initial capture. All structured
requirements derived from it live in `requirements.md`.*
