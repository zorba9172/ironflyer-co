# Execution Unit Economics

Every execution must be priced and measured as a unit of production.

## Revenue

```text
execution_revenue =
platform_fee_allocated
+ execution_credit_spent
+ premium_reasoning_fee
+ sandbox_fee
+ deployment_fee
+ storage_fee
+ mobile_build_fee
```

## Cost

```text
execution_cost =
provider_inference_cost
+ sandbox_compute_cost
+ storage_cost
+ deployment_provider_cost
+ database_event_cost
+ observability_cost
+ support_cost_allocated
+ payment_processing_cost
```

## Gross Profit

```text
gross_profit =
execution_revenue - execution_cost
```

## Gross Margin

```text
gross_margin_pct =
(execution_revenue - execution_cost)
/
execution_revenue
* 100
```

## Hard Rule

If expected gross margin falls below threshold, Profit Guard must:
- degrade model tier
- reduce reasoning depth
- compress context
- reuse blueprint/repair memory
- pause and request budget
- or stop execution
