package episteme

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Process analyses a completed outcome and returns derived signals:
//   - a reputation_input (when the outcome has been human-evaluated), and
//   - improvement proposals + knowledge candidates, derived via an LLM pass over
//     the outcome (skipped when no planner was injected).
//
// It never persists anything — the caller stores the returned signals.
func (e *engine) Process(ctx context.Context, outcome Outcome) ([]Signal, error) {
	signals := make([]Signal, 0, 4)

	if outcome.HumanOutcome != "" {
		signals = append(signals, Signal{
			Kind: KindReputationInput,
			Payload: ReputationInput{
				WorkerID:        outcome.WorkerID,
				HumanOutcome:    outcome.HumanOutcome,
				Confidence:      outcome.Confidence,
				EvidenceQuality: outcome.EvidenceQuality,
			},
		})
	}

	if e.planner == nil {
		return signals, nil
	}

	raw, err := e.planner.Complete(ctx, processPrompt(outcome))
	if err != nil {
		return nil, fmt.Errorf("episteme: process: %w", err)
	}
	proposals, candidates, err := parseProcess(raw)
	if err != nil {
		return nil, fmt.Errorf("episteme: process: parse: %w", err)
	}
	for _, p := range proposals {
		if strings.TrimSpace(p.Proposal) == "" {
			continue
		}
		signals = append(signals, Signal{Kind: KindImprovementProposal, Payload: p})
	}
	for _, c := range candidates {
		if strings.TrimSpace(c.Title) == "" {
			continue
		}
		signals = append(signals, Signal{Kind: KindKnowledgeCandidate, Payload: c})
	}
	return signals, nil
}

func processPrompt(o Outcome) string {
	var b strings.Builder
	b.WriteString("You are an organisational learning analyst reviewing a completed investigation. ")
	b.WriteString("Derive (a) improvement proposals for how the worker could do better next time, and ")
	b.WriteString("(b) reusable knowledge candidates worth remembering.\n\n")
	b.WriteString("Objective: ")
	b.WriteString(o.Objective)
	b.WriteString("\nConclusion: ")
	b.WriteString(o.Conclusion)
	b.WriteString("\nConfidence: ")
	b.WriteString(o.Confidence)
	if o.HumanOutcome != "" {
		b.WriteString("\nHuman evaluation: ")
		b.WriteString(o.HumanOutcome)
	}
	b.WriteString("\n\nEvidence:\n")
	for _, ev := range o.Evidence {
		b.WriteString("- [")
		b.WriteString(ev.Source)
		b.WriteString("] ")
		b.WriteString(ev.Value)
		b.WriteString("\n")
	}
	b.WriteString("\nReturn ONLY JSON of the form:\n")
	b.WriteString(`{"proposals":[{"proposal":"...","rationale":"..."}],"knowledge_candidates":[{"title":"...","content":"..."}]}`)
	return b.String()
}

type processResult struct {
	Proposals           []Proposal           `json:"proposals"`
	KnowledgeCandidates []KnowledgeCandidate `json:"knowledge_candidates"`
}

// parseProcess extracts the JSON object from an LLM reply (tolerating prose or
// code fences around it).
func parseProcess(raw string) ([]Proposal, []KnowledgeCandidate, error) {
	js := extractJSON(raw)
	if js == "" {
		return nil, nil, fmt.Errorf("no JSON object in reply")
	}
	var r processResult
	if err := json.Unmarshal([]byte(js), &r); err != nil {
		return nil, nil, err
	}
	return r.Proposals, r.KnowledgeCandidates, nil
}

// extractJSON returns the substring from the first '{' to the last '}', so a
// reply wrapped in prose or ```json fences still parses.
func extractJSON(s string) string {
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start < 0 || end < 0 || end < start {
		return ""
	}
	return s[start : end+1]
}
