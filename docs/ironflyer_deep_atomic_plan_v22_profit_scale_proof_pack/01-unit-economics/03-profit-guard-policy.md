# Profit Guard Policy

Profit Guard is the economic safety layer.

## Inputs

```json
{
  "execution_id": "exec_01",
  "user_budget_usd": 25,
  "spent_usd": 6.4,
  "reserved_usd": 8.0,
  "estimated_next_step_cost_usd": 1.2,
  "estimated_platform_cost_usd": 3.1,
  "minimum_margin_pct": 45,
  "expected_completion_delta": 0.08,
  "risk_score": 0.22,
  "stop_loss_usd": 12
}
```

## Decisions

```text
continue
degrade
pause_for_budget
stop
kill_branch
switch_provider
reuse_blueprint
reuse_repair
```

## Enforcement Points

- before model call
- before sandbox allocation
- before mobile build
- before Vercel production deploy
- before premium reasoning
- before retry loop
- before long verification
- before storing large artifacts
