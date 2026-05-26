package appsec

type SARIFReport struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema,omitempty"`
	Runs    []SARIFRun `json:"runs"`
}

type SARIFRun struct {
	Tool    SARIFTool     `json:"tool"`
	Results []SARIFResult `json:"results"`
}

type SARIFTool struct {
	Driver SARIFDriver `json:"driver"`
}

type SARIFDriver struct {
	Name           string      `json:"name"`
	InformationURI string      `json:"informationUri,omitempty"`
	Rules          []SARIFRule `json:"rules,omitempty"`
}

type SARIFRule struct {
	ID string `json:"id"`
}

type SARIFResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   SARIFMessage    `json:"message"`
	Locations []SARIFLocation `json:"locations,omitempty"`
}

type SARIFMessage struct {
	Text string `json:"text"`
}

type SARIFLocation struct {
	PhysicalLocation SARIFPhysicalLocation `json:"physicalLocation"`
}

type SARIFPhysicalLocation struct {
	ArtifactLocation SARIFArtifactLocation `json:"artifactLocation"`
	Region           SARIFRegion           `json:"region,omitempty"`
}

type SARIFArtifactLocation struct {
	URI string `json:"uri"`
}

type SARIFRegion struct {
	StartLine int `json:"startLine,omitempty"`
}

func ToSARIF(result Result) SARIFReport {
	rulesSeen := map[string]bool{}
	var rules []SARIFRule
	var results []SARIFResult
	for _, f := range result.Findings {
		if !rulesSeen[f.RuleID] {
			rulesSeen[f.RuleID] = true
			rules = append(rules, SARIFRule{ID: f.RuleID})
		}
		res := SARIFResult{
			RuleID:  f.RuleID,
			Level:   sarifLevel(f.Severity),
			Message: SARIFMessage{Text: firstNonEmpty(f.Summary, f.RuleID)},
		}
		if f.Path != "" {
			res.Locations = []SARIFLocation{{
				PhysicalLocation: SARIFPhysicalLocation{
					ArtifactLocation: SARIFArtifactLocation{URI: f.Path},
					Region:           SARIFRegion{StartLine: f.Line},
				},
			}}
		}
		results = append(results, res)
	}
	return SARIFReport{
		Version: "2.1.0",
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Runs: []SARIFRun{{
			Tool: SARIFTool{Driver: SARIFDriver{
				Name:           "Ironflyer AppSec",
				InformationURI: "https://ironflyer.dev",
				Rules:          rules,
			}},
			Results: results,
		}},
	}
}

func sarifLevel(sev Severity) string {
	switch sev {
	case SeverityCritical, SeverityHigh:
		return "error"
	case SeverityMedium:
		return "warning"
	default:
		return "note"
	}
}
