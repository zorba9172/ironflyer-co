# Margin Thresholds

## Default Minimum Gross Margin

| Workload | Minimum Gross Margin |
|---|---:|
| Standard web execution | 45% |
| Premium reasoning | 55% |
| Sandbox runtime | 45% |
| Vercel preview | 30% |
| Mobile build | 35% |
| Storage | 50% |
| Enterprise governance | 70% |
| Support-heavy execution | 60% |

## Stop-Loss Rules

- no free execution can exceed free credit cap
- no paid execution can exceed user budget without approval
- no retry branch can exceed expected ROI threshold
- no sandbox remains alive without active progress
- no premium model call without expected completion delta
