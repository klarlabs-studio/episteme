package episteme

import "context"

// Reputation computes a worker's reputation dimensions from its signal history
// (the reputation_input signals, in chronological order). Below minSignals it
// returns a zero Profile — too little ground truth to score.
//
// Scoring (verbatim from the platform):
//   accuracy               = correct / total
//   evidence_quality       = avg(high=1.0, medium=0.5, low=0.0) over inputs whose
//                            EvidenceQuality is one of high/medium/low (others excluded)
//   confidence_calibration = high_confidence_correct / high_confidence_total
//   overall                = accuracy*0.5 + evidence_quality*0.3 + calibration*0.2
func (e *engine) Reputation(_ context.Context, signals []Signal) (Profile, error) {
	inputs := reputationInputs(signals)
	if len(inputs) < minSignals {
		return Profile{Dimensions: map[string]DimensionScore{}}, nil
	}

	accuracy, evidence, calibration, overall, samples := score(inputs)

	// Trend per dimension: compare the score now against the score before the most
	// recent input (mirrors comparing the two latest history snapshots).
	prevAcc, prevEv, prevCal, prevOverall, _ := score(inputs[:len(inputs)-1])

	dims := map[string]DimensionScore{
		DimAccuracy:    {Score: accuracy, SampleSize: samples.total, Trend: trend(accuracy, prevAcc)},
		DimEvidence:    {Score: evidence, SampleSize: samples.evCount, Trend: trend(evidence, prevEv)},
		DimCalibration: {Score: calibration, SampleSize: samples.highTotal, Trend: trend(calibration, prevCal)},
	}
	_ = prevOverall
	return Profile{
		Dimensions: dims,
		Overall:    overall,
	}, nil
}

type sampleCounts struct {
	total     int
	evCount   int
	highTotal int
}

// score computes the four scores + sample counts for a set of inputs.
func score(inputs []ReputationInput) (accuracy, evidence, calibration, overall float64, samples sampleCounts) {
	if len(inputs) == 0 {
		return 0, 0, 0, 0, sampleCounts{}
	}
	correct, highTotal, highCorrect := 0, 0, 0
	evSum, evCount := 0.0, 0
	for _, in := range inputs {
		isCorrect := in.HumanOutcome == "correct"
		if isCorrect {
			correct++
		}
		switch in.EvidenceQuality {
		case "high":
			evSum += 1.0
			evCount++
		case "medium":
			evSum += 0.5
			evCount++
		case "low":
			evCount++
		}
		if in.Confidence == "high" {
			highTotal++
			if isCorrect {
				highCorrect++
			}
		}
	}
	accuracy = float64(correct) / float64(len(inputs))
	if evCount > 0 {
		evidence = evSum / float64(evCount)
	}
	if highTotal > 0 {
		calibration = float64(highCorrect) / float64(highTotal)
	}
	overall = accuracy*wAccuracy + evidence*wEvidence + calibration*wCalibration
	return accuracy, evidence, calibration, overall, sampleCounts{total: len(inputs), evCount: evCount, highTotal: highTotal}
}

func trend(now, prev float64) string {
	switch {
	case now > prev+trendDeadband:
		return "improving"
	case now < prev-trendDeadband:
		return "declining"
	default:
		return "stable"
	}
}

// reputationInputs extracts the ReputationInput payloads from a signal slice.
// Signals of other kinds are ignored, so callers can pass a mixed history.
func reputationInputs(signals []Signal) []ReputationInput {
	out := make([]ReputationInput, 0, len(signals))
	for _, s := range signals {
		if s.Kind != KindReputationInput {
			continue
		}
		if in, ok := s.Payload.(ReputationInput); ok {
			out = append(out, in)
		}
	}
	return out
}
