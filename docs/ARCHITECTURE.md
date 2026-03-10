# agentmeter — Architecture

## Components

```
┌─────────────────────────────────────────────────────┐
│  Meter  (mutable, thread-safe)                      │
│                                                     │
│  cfg: CostFunc, MaxHistory                          │
│  run: current runState (steps, summary, timing)     │
│  hist: []Snapshot  (bounded, completed runs)        │
└──────────────────────┬──────────────────────────────┘
                       │  Snapshot() / Finalize()
                       ▼
┌─────────────────────────────────────────────────────┐
│  Snapshot  (immutable, deep copy)                   │
│                                                     │
│  Steps []AgentStep    — full trace                  │
│  TokenSummary         — per-model usage + cost      │
│  TotalDuration        — wall-clock elapsed          │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│  Printer  (rendering)                               │
│                                                     │
│  Print(snap)          — step-by-step trace          │
│  PrintTokenSummary()  — token/cost totals           │
└─────────────────────────────────────────────────────┘
```

---

## Run Lifecycle

```
New() ──► Reset(label) ──► Record() × N ──► Finalize()
                                                 │
                                           appended to hist
                                           (Snapshot.Label = label)
                                                 │
               Reset(label) ◄────── start next run ◄┘
```

Each call to `Reset` clears the current run state but leaves history intact.
`Finalize` is idempotent: calling it more than once is a no-op.

---

## Run Label vs Agent Name

Two distinct identifiers serve different purposes:

| Field | Scope | Set by | Meaning |
|---|---|---|---|
| `Snapshot.Label` | Run | `Reset(label)` | Identifies the run or session |
| `AgentStep.AgentName` | Step | Caller in `Record()` | Identifies which agent produced this step |

`Meter` never writes `AgentStep.AgentName` — it is entirely the caller's responsibility. This lets multiple agents (planner, executor, retriever) record into the same `Meter` while keeping their identity per step.

---

## Record() Internals

For each `AgentStep`:

1. Duration auto-computed from `StartedAt` if `Duration == 0`
2. Token usage added to `ByModel[ModelID]` (if `ModelID != ""`)
3. `ModelCalls` incremented if `Cluster == ClusterCognitive`
4. Step appended to `run.steps`

---

## Step Timing

**Recommended — set `StartedAt` before the blocking call:**

```go
start := time.Now()
resp  := callModel(...)
meter.Record(agentmeter.AgentStep{
    StartedAt: start,  // Record() computes time.Since(start)
})
```

`StartedAt` travels with the step value — there is no shared timing state on `Meter`, so concurrent goroutines cannot corrupt each other's durations.

**Alternative — provide `Duration` directly:**

```go
meter.Record(agentmeter.AgentStep{
    Duration: measuredElsewhere,  // used as-is
})
```

### TotalDuration

`Snapshot.TotalDuration` is the wall-clock time from `Reset()` to `Finalize()`. It covers the entire run, including gaps between steps. For a snapshot taken before `Finalize()`, it is computed live as `time.Since(startTime)`.

---

## History (bounded retention)

History is a slice capped at `MaxHistory` (default 100). When the limit is exceeded, the oldest entry is dropped:

```
hist = append(hist, snap)
if len(hist) > max {
    hist[0] = Snapshot{}  // release GC reference
    hist = hist[1:]
}
```

`History()` returns deep copies. `ClearHistory()` sets `hist = nil`.

---

## Concurrency

- `Meter` uses `*sync.Mutex` (pointer, to avoid `copylocks` vet failure on value copies).
- All public methods lock before touching `run` or `hist`.
- `cfg` is read-only after `New()` and never locked.
- `CostFunc` is called while holding the lock — keep it fast and pure.