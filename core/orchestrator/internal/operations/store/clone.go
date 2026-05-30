package store

import (
	"encoding/json"

	"ironflyer/core/orchestrator/internal/ai/domain"
)

func cloneProject(p domain.Project) domain.Project {
	p.Spec = cloneProductSpec(p.Spec)
	p.Files = cloneSlice(p.Files)
	p.Artifacts = cloneArtifacts(p.Artifacts)
	p.Gates = cloneGates(p.Gates)
	p.Events = cloneSlice(p.Events)
	if p.GitHub != nil {
		g := *p.GitHub
		p.GitHub = &g
	}
	p.Secrets = cloneStringMap(p.Secrets)
	p.VisualTargets = cloneSlice(p.VisualTargets)
	p.Subprojects = cloneSubprojects(p.Subprojects)
	return p
}

func cloneProductSpec(s domain.ProductSpec) domain.ProductSpec {
	s.UserStories = cloneSlice(s.UserStories)
	for i := range s.UserStories {
		s.UserStories[i].Acceptance = cloneSlice(s.UserStories[i].Acceptance)
	}
	s.DataModel = cloneSlice(s.DataModel)
	for i := range s.DataModel {
		s.DataModel[i].Fields = cloneSlice(s.DataModel[i].Fields)
	}
	s.Stack = cloneStackDecision(s.Stack)
	s.Compliance = cloneSlice(s.Compliance)
	return s
}

func cloneStackDecision(s domain.StackDecision) domain.StackDecision {
	s.Mobile.Targets = cloneSlice(s.Mobile.Targets)
	if s.Mobile.EAS != nil {
		eas := *s.Mobile.EAS
		s.Mobile.EAS = &eas
	}
	if s.Mobile.Signing != nil {
		signing := *s.Mobile.Signing
		s.Mobile.Signing = &signing
	}
	return s
}

func cloneArtifacts(in map[string]json.RawMessage) map[string]json.RawMessage {
	if in == nil {
		return nil
	}
	out := make(map[string]json.RawMessage, len(in))
	for k, v := range in {
		out[k] = cloneRawMessage(v)
	}
	return out
}

func cloneGates(in map[domain.GateName]domain.GateState) map[domain.GateName]domain.GateState {
	if in == nil {
		return nil
	}
	out := make(map[domain.GateName]domain.GateState, len(in))
	for k, v := range in {
		v.Issues = cloneSlice(v.Issues)
		out[k] = v
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneSubprojects(in []domain.Subproject) []domain.Subproject {
	out := cloneSlice(in)
	for i := range out {
		out[i].Stack = cloneStackDecision(out[i].Stack)
	}
	return out
}

func cloneSlice[T any](in []T) []T {
	if in == nil {
		return nil
	}
	out := make([]T, len(in))
	copy(out, in)
	return out
}

func cloneRawMessage(in json.RawMessage) json.RawMessage {
	if in == nil {
		return nil
	}
	out := make(json.RawMessage, len(in))
	copy(out, in)
	return out
}
