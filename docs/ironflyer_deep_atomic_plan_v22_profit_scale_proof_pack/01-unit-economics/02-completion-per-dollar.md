# Completion Per Dollar

Completion-per-dollar is the core runtime efficiency metric.

```text
completion_per_dollar =
completion_score_delta / execution_cost_usd
```

## Example

If completion score improves from 0.30 to 0.70 and the platform cost is $2:

```text
completion_per_dollar = (0.70 - 0.30) / 2 = 0.20
```

The system uses this to decide whether to continue, degrade, or stop.

## Runtime Decisions

Continue when:
```text
expected_completion_delta / expected_cost >= threshold
```

Degrade when:
```text
expected_completion_delta positive
but margin below threshold
```

Stop when:
```text
expected_completion_delta low
and cost/risk high
```

Ask for budget when:
```text
execution has high completion probability
but user budget is insufficient
```
