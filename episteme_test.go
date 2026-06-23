package episteme

import (
	"context"
	"math"
	"strings"
	"testing"
)

// fakePlanner routes by prompt content so replay, judge, and process are
// deterministic in tests.
type fakePlanner struct {
	process string // canned JSON for the process prompt
	err     error
}

func (f fakePlanner) Complete(_ context.Context, prompt string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	switch {
	case strings.Contains(prompt, "grading"): // judgePrompt
		if strings.Contains(prompt, "good-conclusion") {
			return "equivalent", nil
		}
		if strings.Contains(prompt, "partial-conclusion") {
			return "partial", nil
		}
		return "wrong", nil
	case strings.Contains(prompt, "REPLAY MODE"): // replayPrompt
		switch {
		case strings.Contains(prompt, "GOOD"):
			return "good-conclusion", nil
		case strings.Contains(prompt, "PARTIAL"):
			return "partial-conclusion", nil
		default:
			return "bad-conclusion", nil
		}
	default: // process prompt
		return f.process, nil
	}
}

func repInput(outcome, conf, ev string) Signal {
	return Signal{Kind: KindReputationInput, Payload: ReputationInput{HumanOutcome: outcome, Confidence: conf, EvidenceQuality: ev}}
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestReputationBelowMinSignals(t *testing.T) {
	p, err := New(nil).Reputation(context.Background(), []Signal{repInput("correct", "high", "high"), repInput("correct", "high", "high")})
	if err != nil {
		t.Fatal(err)
	}
	if p.Overall != 0 || len(p.Dimensions) != 0 {
		t.Fatalf("expected empty profile below minSignals, got %+v", p)
	}
}

func TestReputationGolden(t *testing.T) {
	signals := []Signal{
		repInput("correct", "high", "high"),
		repInput("correct", "high", "medium"),
		repInput("incorrect", "medium", "low"),
	}
	p, err := New(nil).Reputation(context.Background(), signals)
	if err != nil {
		t.Fatal(err)
	}
	wantAcc := 2.0 / 3.0
	wantEv := 0.5  // (1.0 + 0.5 + 0.0) / 3
	wantCal := 1.0 // 2 high-correct / 2 high-total
	wantOverall := wantAcc*0.5 + wantEv*0.3 + wantCal*0.2

	if !approx(p.Dimensions[DimAccuracy].Score, wantAcc) {
		t.Errorf("accuracy = %v, want %v", p.Dimensions[DimAccuracy].Score, wantAcc)
	}
	if !approx(p.Dimensions[DimEvidence].Score, wantEv) {
		t.Errorf("evidence = %v, want %v", p.Dimensions[DimEvidence].Score, wantEv)
	}
	if !approx(p.Dimensions[DimCalibration].Score, wantCal) {
		t.Errorf("calibration = %v, want %v", p.Dimensions[DimCalibration].Score, wantCal)
	}
	if !approx(p.Overall, wantOverall) {
		t.Errorf("overall = %v, want %v", p.Overall, wantOverall)
	}
	if p.Dimensions[DimAccuracy].SampleSize != 3 {
		t.Errorf("accuracy samples = %d, want 3", p.Dimensions[DimAccuracy].SampleSize)
	}
	if p.Dimensions[DimCalibration].SampleSize != 2 {
		t.Errorf("calibration samples = %d, want 2", p.Dimensions[DimCalibration].SampleSize)
	}
	// Adding the incorrect/low third input drops accuracy and evidence vs the
	// first two → declining; calibration unchanged (1.0) → stable.
	if got := p.Dimensions[DimAccuracy].Trend; got != "declining" {
		t.Errorf("accuracy trend = %q, want declining", got)
	}
	if got := p.Dimensions[DimCalibration].Trend; got != "stable" {
		t.Errorf("calibration trend = %q, want stable", got)
	}
}

func TestReputationIgnoresNonReputationSignals(t *testing.T) {
	signals := []Signal{
		{Kind: KindImprovementProposal, Payload: Proposal{Proposal: "x"}},
		repInput("correct", "high", "high"),
		repInput("correct", "low", "medium"),
		repInput("partial", "medium", "low"),
	}
	p, err := New(nil).Reputation(context.Background(), signals)
	if err != nil {
		t.Fatal(err)
	}
	if p.Dimensions[DimAccuracy].SampleSize != 3 {
		t.Fatalf("expected 3 reputation inputs counted, got %d", p.Dimensions[DimAccuracy].SampleSize)
	}
}

func TestEvaluateRecommendations(t *testing.T) {
	scen := []Scenario{{Objective: "o", Evidence: []EvidenceItem{{Source: "s", Value: "v"}}, ExpectedConclusion: "ok"}}
	cases := []struct {
		name, cand, base, want string
	}{
		{"activate", "GOOD", "BAD", "activate"},
		{"reject", "BAD", "GOOD", "reject"},
		{"inconclusive", "GOOD", "GOOD", "inconclusive"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rep, err := New(nil).Evaluate(context.Background(), EvaluationRequest{
				CandidateSystemPrompt: tc.cand,
				BaselineSystemPrompt:  tc.base,
				Scenarios:             scen,
				Planner:               fakePlanner{},
			})
			if err != nil {
				t.Fatal(err)
			}
			if rep.Recommendation != tc.want {
				t.Errorf("recommendation = %q, want %q (cand=%v base=%v)", rep.Recommendation, tc.want, rep.CandidateScore, rep.BaselineScore)
			}
		})
	}
}

func TestEvaluateNoPlanner(t *testing.T) {
	_, err := New(nil).Evaluate(context.Background(), EvaluationRequest{Scenarios: []Scenario{{}}})
	if err != ErrNoPlanner {
		t.Fatalf("want ErrNoPlanner, got %v", err)
	}
}

func TestEvaluateNoScenarios(t *testing.T) {
	rep, err := New(fakePlanner{}).Evaluate(context.Background(), EvaluationRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Recommendation != "inconclusive" {
		t.Fatalf("want inconclusive for zero scenarios, got %q", rep.Recommendation)
	}
}

func TestProcessReputationInputOnlyWithoutPlanner(t *testing.T) {
	sigs, err := New(nil).Process(context.Background(), Outcome{WorkerID: "w", HumanOutcome: "correct", Confidence: "high", EvidenceQuality: "high"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sigs) != 1 || sigs[0].Kind != KindReputationInput {
		t.Fatalf("want one reputation_input, got %+v", sigs)
	}
}

func TestProcessDerivesProposalsAndKnowledge(t *testing.T) {
	planner := fakePlanner{process: "Here you go:\n```json\n" +
		`{"proposals":[{"proposal":"always summarise","rationale":"clarity"},{"proposal":""}],` +
		`"knowledge_candidates":[{"title":"prom tip","content":"check up=1"},{"title":""}]}` +
		"\n```"}
	sigs, err := New(planner).Process(context.Background(), Outcome{
		WorkerID: "w", HumanOutcome: "correct", Confidence: "high", EvidenceQuality: "high",
		Objective: "investigate", Conclusion: "healthy",
		Evidence: []EvidenceItem{{Source: "query_metrics", Value: "up=1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var rep, prop, know int
	for _, s := range sigs {
		switch s.Kind {
		case KindReputationInput:
			rep++
		case KindImprovementProposal:
			prop++
		case KindKnowledgeCandidate:
			know++
		}
	}
	if rep != 1 || prop != 1 || know != 1 {
		t.Fatalf("want 1/1/1 reputation/proposal/knowledge (empty entries skipped), got %d/%d/%d", rep, prop, know)
	}
}

func TestExtractJSON(t *testing.T) {
	if got := extractJSON("noise {\"a\":1} trailing"); got != `{"a":1}` {
		t.Fatalf("extractJSON = %q", got)
	}
	if got := extractJSON("no json here"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}
