# Base Case Simulation

Assumptions:

```text
paying_users = 500
executions_per_user_month = 4
avg_user_charge_per_execution = $18
avg_platform_cost_per_execution = $6.30
refund_rate = 5%
completion_or_preview_success = 80%
```

## Calculations

```text
monthly_executions = 500 * 4 = 2,000

gross_revenue = 2,000 * $18 = $36,000

refunds = $36,000 * 5% = $1,800

net_revenue = $34,200

platform_cost = 2,000 * $6.30 = $12,600

gross_profit = $34,200 - $12,600 = $21,600

gross_margin = $21,600 / $34,200 = 63.1%
```

## Interpretation

This is attractive only if:
- support cost stays low
- completion/preview success stays high
- users repeat
- free usage is tightly capped
