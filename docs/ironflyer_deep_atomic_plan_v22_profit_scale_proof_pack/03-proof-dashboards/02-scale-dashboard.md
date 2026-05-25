# Scale Dashboard

## Metrics

- active executions
- queued paid executions
- queue wait time
- sandbox capacity
- runtime worker utilization
- Redpanda lag
- Scylla ingestion pressure
- ClickHouse lag
- Temporal workflow failures
- zombie rate
- recovery success
- provider latency
- Vercel deploy success

## Scale Health

```text
scale_health =
capacity_available
* recovery_success
* event_lag_health
* margin_health
```

## Rule

If paid demand rises but margin drops, scaling is unhealthy.
