package appsec

type NodeKind string

const (
	NodeProject NodeKind = "project"
	NodeService NodeKind = "service"
	NodeFile    NodeKind = "file"
	NodePackage NodeKind = "package"
	NodeFinding NodeKind = "finding"
)

type RiskNode struct {
	ID       string
	Kind     NodeKind
	Label    string
	Category string
	Severity Severity
}

type RiskEdge struct {
	From string
	To   string
	Kind string
}

type RiskGraph struct {
	Nodes []RiskNode
	Edges []RiskEdge
}

func BuildRiskGraph(target Target, inv Inventory, findings []Finding) RiskGraph {
	projectID := target.ProjectID
	if projectID == "" {
		projectID = "project"
	}
	graph := RiskGraph{
		Nodes: []RiskNode{{ID: "project:" + projectID, Kind: NodeProject, Label: projectID}},
	}
	for _, svc := range inv.Services {
		id := "service:" + svc.ID
		graph.Nodes = append(graph.Nodes, RiskNode{ID: id, Kind: NodeService, Label: svc.Path, Category: svc.Language})
		graph.Edges = append(graph.Edges, RiskEdge{From: "project:" + projectID, To: id, Kind: "contains"})
	}
	for _, c := range inv.Components {
		id := "package:" + c.Ecosystem + ":" + c.Name + "@" + c.Version
		graph.Nodes = append(graph.Nodes, RiskNode{ID: id, Kind: NodePackage, Label: c.Name + "@" + c.Version, Category: c.Ecosystem})
		graph.Edges = append(graph.Edges, RiskEdge{From: "project:" + projectID, To: id, Kind: "depends_on"})
	}
	for _, f := range findings {
		findingID := "finding:" + f.ID
		graph.Nodes = append(graph.Nodes, RiskNode{
			ID:       findingID,
			Kind:     NodeFinding,
			Label:    f.RuleID,
			Category: string(f.Category),
			Severity: f.Severity,
		})
		if f.Path != "" {
			fileID := "file:" + f.Path
			graph.Nodes = append(graph.Nodes, RiskNode{ID: fileID, Kind: NodeFile, Label: f.Path})
			graph.Edges = append(graph.Edges, RiskEdge{From: findingID, To: fileID, Kind: "affects"})
		}
		if f.Package != "" {
			pkgID := "package:" + f.Package
			graph.Nodes = append(graph.Nodes, RiskNode{ID: pkgID, Kind: NodePackage, Label: f.Package})
			graph.Edges = append(graph.Edges, RiskEdge{From: findingID, To: pkgID, Kind: "affects"})
		}
		graph.Edges = append(graph.Edges, RiskEdge{From: "project:" + projectID, To: findingID, Kind: "has_finding"})
	}
	return graph
}
