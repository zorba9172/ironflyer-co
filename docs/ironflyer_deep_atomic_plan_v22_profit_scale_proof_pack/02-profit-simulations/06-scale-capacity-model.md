# Scale Capacity Model

## Execution Capacity Formula

```text
max_concurrent_executions =
available_runtime_workers
*
avg_executions_per_worker
*
stability_factor
```

## Example

```text
runtime_workers = 100
avg_executions_per_worker = 4
stability_factor = 0.75

max_concurrent_executions = 300
```

## Monthly Capacity

```text
monthly_execution_capacity =
concurrent_executions
*
executions_per_day_per_slot
*
30
```

If:
```text
300 concurrent slots
6 executions/day/slot
30 days
```

Then:
```text
monthly_capacity = 54,000 executions/month
```

## Scale Rule

Do not scale workers unless:
- paid queue exists
- budget is reserved
- expected margin is positive
