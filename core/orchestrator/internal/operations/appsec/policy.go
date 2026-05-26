package appsec

type Policy struct {
	BlockOnHigh       bool
	BlockOnMedium     bool
	MaxFindingsPerRun int
}

func DefaultPolicy() Policy {
	return Policy{
		BlockOnHigh:       true,
		BlockOnMedium:     false,
		MaxFindingsPerRun: 200,
	}
}

func (p Policy) Evaluate(findings []Finding) Verdict {
	if p.MaxFindingsPerRun <= 0 {
		p.MaxFindingsPerRun = DefaultPolicy().MaxFindingsPerRun
	}
	worst := SeverityInfo
	weighted := 0.0
	blocked := false
	for _, f := range findings {
		switch f.Severity {
		case SeverityCritical:
			weighted += 0.50
			worst = maxSeverity(worst, f.Severity)
			blocked = true
		case SeverityHigh:
			weighted += 0.30
			worst = maxSeverity(worst, f.Severity)
			if p.BlockOnHigh {
				blocked = true
			}
		case SeverityMedium:
			weighted += 0.15
			worst = maxSeverity(worst, f.Severity)
			if p.BlockOnMedium {
				blocked = true
			}
		case SeverityLow:
			weighted += 0.05
			worst = maxSeverity(worst, f.Severity)
		}
	}
	status := "pass"
	switch worst {
	case SeverityCritical, SeverityHigh:
		status = "fail"
	case SeverityMedium, SeverityLow:
		status = "warning"
	}
	score := 1.0 - weighted/4.0
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return Verdict{Status: status, BlockedDeploy: blocked, OverallScore: score}
}

func maxSeverity(a, b Severity) Severity {
	if severityRank(b) > severityRank(a) {
		return b
	}
	return a
}

func severityRank(s Severity) int {
	switch s {
	case SeverityCritical:
		return 5
	case SeverityHigh:
		return 4
	case SeverityMedium:
		return 3
	case SeverityLow:
		return 2
	default:
		return 1
	}
}
