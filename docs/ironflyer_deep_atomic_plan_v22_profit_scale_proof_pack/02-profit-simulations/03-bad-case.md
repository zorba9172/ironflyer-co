# Bad Case Simulation

Assumptions:

```text
paying_users = 500
executions_per_user_month = 4
avg_user_charge_per_execution = $18
avg_platform_cost_per_execution = $15
refund_rate = 12%
completion_or_preview_success = 55%
```

## Calculations

```text
monthly_executions = 2,000
gross_revenue = $36,000
refunds = $4,320
net_revenue = $31,680
platform_cost = $30,000
gross_profit = $1,680
gross_margin = 5.3%
```

## Interpretation

This business is unhealthy.

Causes:
- too many frontier model calls
- no blueprint reuse
- too many retries
- weak completion
- support burden likely high
- user trust likely weak

Profit Guard must prevent this mode.
