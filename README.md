# episteme

The organisational learning engine extracted from the Senat platform. A pure
reasoning core: it processes completed units of work and derives intelligence —
improvement proposals, knowledge candidates, reputation scores, and version
evaluations.

Storage is the caller's concern. episteme operates on signals and values, never
on a database. It depends on **no** Klarlabs library; the only injected seam is
`Planner` (an LLM: `Complete(ctx, prompt) (string, error)`).

```go
eng := episteme.New(planner)           // planner may be nil if only Reputation is used
profile, _ := eng.Reputation(ctx, signals)
signals, _ := eng.Process(ctx, outcome)
report, _  := eng.Evaluate(ctx, episteme.EvaluationRequest{...})
```

See `go.klarlabs.de/episteme` docs. Spec: Senat-OS `library-specs/spec-episteme.md`.
