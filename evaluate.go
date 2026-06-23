package episteme

import (
	"context"
	"fmt"
	"strings"
)

// Evaluate scores a candidate system prompt against a baseline over the given
// scenarios in replay mode (no live tool calls): for each scenario the system
// prompt + frozen evidence are rendered into a replay prompt, a conclusion is
// produced via the Planner, then judged against the scenario's expected
// conclusion (1.0 equivalent / 0.5 partial / 0.0 wrong). The recommendation is
// advisory — episteme never activates anything.
func (e *engine) Evaluate(ctx context.Context, req EvaluationRequest) (Report, error) {
	planner := req.Planner
	if planner == nil {
		planner = e.planner
	}
	if planner == nil {
		return Report{}, ErrNoPlanner
	}
	if len(req.Scenarios) == 0 {
		return Report{Recommendation: "inconclusive"}, nil
	}

	scores := make([]ScenarioScore, 0, len(req.Scenarios))
	var candSum, baseSum float64
	for _, sc := range req.Scenarios {
		cand, err := e.replayScore(ctx, planner, req.CandidateSystemPrompt, sc)
		if err != nil {
			return Report{}, err
		}
		base, err := e.replayScore(ctx, planner, req.BaselineSystemPrompt, sc)
		if err != nil {
			return Report{}, err
		}
		scores = append(scores, ScenarioScore{Candidate: cand, Baseline: base})
		candSum += cand
		baseSum += base
	}
	n := float64(len(req.Scenarios))
	candidateScore := candSum / n
	baselineScore := baseSum / n

	return Report{
		CandidateScore: candidateScore,
		BaselineScore:  baselineScore,
		Recommendation: recommend(candidateScore, baselineScore),
		ScenarioScores: scores,
	}, nil
}

func recommend(candidate, baseline float64) string {
	switch {
	case candidate >= baseline+activateMargin:
		return "activate"
	case candidate < baseline-rejectMargin:
		return "reject"
	default:
		return "inconclusive"
	}
}

// replayScore renders + replays one scenario under a system prompt and judges the
// produced conclusion against the expected one.
func (e *engine) replayScore(ctx context.Context, planner Planner, systemPrompt string, sc Scenario) (float64, error) {
	conclusion, err := planner.Complete(ctx, replayPrompt(systemPrompt, sc))
	if err != nil {
		return 0, fmt.Errorf("episteme: replay: %w", err)
	}
	return judge(ctx, planner, sc.ExpectedConclusion, conclusion)
}

func replayPrompt(systemPrompt string, sc Scenario) string {
	var b strings.Builder
	b.WriteString(systemPrompt)
	b.WriteString("\n\n--- REPLAY MODE ---\n")
	b.WriteString("You are replaying a past investigation. You may NOT call any tools. ")
	b.WriteString("Reach a conclusion using ONLY the frozen evidence below.\n\n")
	b.WriteString("Objective: ")
	b.WriteString(sc.Objective)
	b.WriteString("\n\nEvidence:\n")
	for _, ev := range sc.Evidence {
		b.WriteString("- [")
		b.WriteString(ev.Source)
		b.WriteString("] ")
		b.WriteString(ev.Value)
		b.WriteString("\n")
	}
	b.WriteString("\nState your conclusion concisely.")
	return b.String()
}

// judge asks the planner to compare a produced conclusion against the expected
// one, returning 1.0 (equivalent), 0.5 (partial), or 0.0 (wrong). A reply that
// does not clearly indicate equivalent/partial is scored 0.0.
func judge(ctx context.Context, planner Planner, expected, produced string) (float64, error) {
	verdict, err := planner.Complete(ctx, judgePrompt(expected, produced))
	if err != nil {
		return 0, fmt.Errorf("episteme: judge: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(verdict)) {
	case "equivalent":
		return 1.0, nil
	case "partial":
		return 0.5, nil
	default:
		return 0.0, nil
	}
}

func judgePrompt(expected, produced string) string {
	return "You are grading whether a produced conclusion matches an expected one.\n\n" +
		"Expected conclusion:\n" + expected + "\n\n" +
		"Produced conclusion:\n" + produced + "\n\n" +
		"Reply with exactly one word: 'equivalent' (same root cause / answer), " +
		"'partial' (related but incomplete or hedged), or 'wrong' (different or incorrect)."
}
