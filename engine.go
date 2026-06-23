package episteme

import "errors"

// Recommendation thresholds + scoring weights, lifted from the shipped Senat
// implementation so the library matches proven behaviour.
const (
	minSignals = 3 // below this, reputation is not meaningful

	wAccuracy    = 0.5
	wEvidence    = 0.3
	wCalibration = 0.2

	activateMargin = 0.05 // candidate >= baseline + this → activate
	rejectMargin   = 0.05 // candidate <  baseline - this → reject
	trendDeadband  = 0.01
)

// ErrNoPlanner is returned when a method needs an LLM but none was injected.
var ErrNoPlanner = errors.New("episteme: no planner provided")

type engine struct {
	planner Planner
}

// New constructs the default Engine. planner is used by Process (and by Evaluate
// when EvaluationRequest.Planner is nil); it may be nil if only Reputation is used.
func New(planner Planner) Engine {
	return &engine{planner: planner}
}
