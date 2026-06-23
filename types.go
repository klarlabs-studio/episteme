// Package episteme is the organisational learning engine extracted from the
// Senat platform. It processes completed units of work and derives intelligence
// from them: improvement proposals, knowledge candidates, reputation scores, and
// version evaluations.
//
// Storage is the caller's concern. episteme operates on signals and values,
// never on tables or a database — it is a pure reasoning core. It depends on no
// Klarlabs library; the only injected seam is Planner (an LLM).
package episteme

import "context"

// Engine is the organisational learning engine. All methods are pure with
// respect to storage — they read their inputs from the arguments and return
// derived artifacts; persistence is the caller's job.
type Engine interface {
	// Process analyses a completed run outcome and returns derived signals:
	// improvement proposals, knowledge candidates, and a reputation input.
	Process(ctx context.Context, outcome Outcome) ([]Signal, error)

	// Reputation computes reputation dimensions from a worker's signal history.
	Reputation(ctx context.Context, signals []Signal) (Profile, error)

	// Evaluate scores a candidate configuration against a set of scenarios using
	// replay mode — no live tool calls.
	Evaluate(ctx context.Context, req EvaluationRequest) (Report, error)
}

// Planner is the LLM seam episteme uses for replay, judging, and proposal
// derivation. The caller injects any LLM.
type Planner interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

// Signal kinds.
const (
	KindImprovementProposal = "improvement_proposal"
	KindKnowledgeCandidate  = "knowledge_candidate"
	KindReputationInput     = "reputation_input"
)

// Outcome represents a completed unit of work.
type Outcome struct {
	WorkerID        string
	OrgID           string
	Objective       string
	Evidence        []EvidenceItem
	Conclusion      string
	Confidence      string // high | medium | low
	EvidenceQuality string // high | medium | low
	HumanOutcome    string // correct | partial | incorrect — empty if not yet evaluated
}

// EvidenceItem is one piece of the evidence trail an outcome was built on.
type EvidenceItem struct {
	Kind   string // tool name / evidence kind
	Source string // where it came from (e.g. "search_github")
	Value  string // what it returned
}

// Signal is a derived learning artifact from an outcome.
type Signal struct {
	Kind    string
	Payload any
}

// ReputationInput is the Signal.Payload for Kind == KindReputationInput, and the
// element type Reputation consumes.
type ReputationInput struct {
	WorkerID        string
	HumanOutcome    string // correct | partial | incorrect
	Confidence      string // high | medium | low
	EvidenceQuality string // high | medium | low
}

// Proposal is the Signal.Payload for Kind == KindImprovementProposal.
type Proposal struct {
	Proposal  string
	Rationale string
}

// KnowledgeCandidate is the Signal.Payload for Kind == KindKnowledgeCandidate.
type KnowledgeCandidate struct {
	Title   string
	Content string
}

// Reputation dimension names.
const (
	DimAccuracy    = "accuracy"
	DimEvidence    = "evidence_quality"
	DimCalibration = "confidence_calibration"
)

// Profile is a worker's reputation across dimensions.
type Profile struct {
	Dimensions map[string]DimensionScore // accuracy | evidence_quality | confidence_calibration
	Overall    float64
}

// DimensionScore is one reputation dimension.
type DimensionScore struct {
	Score      float64
	SampleSize int
	Trend      string // improving | stable | declining
}

// Scenario is a replayable test case: objective + frozen evidence + expected
// conclusion (a known-correct historical outcome).
type Scenario struct {
	Objective          string
	Evidence           []EvidenceItem
	ExpectedConclusion string
	ExpectedConfidence string
}

// EvaluationRequest asks the engine to compare a candidate against a baseline.
type EvaluationRequest struct {
	CandidateSystemPrompt string
	BaselineSystemPrompt  string
	Scenarios             []Scenario
	Planner               Planner // LLM for replay and judging; falls back to the engine's planner if nil
}

// Report is the result of a version evaluation.
type Report struct {
	CandidateScore float64
	BaselineScore  float64
	Recommendation string // activate | reject | inconclusive
	ScenarioScores []ScenarioScore
}

// ScenarioScore is the per-scenario candidate/baseline pair.
type ScenarioScore struct {
	Candidate float64
	Baseline  float64
}
