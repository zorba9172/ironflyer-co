package resolver

// Helpers for the devicecloud resolver. Lives in a separate file so the
// next `gqlgen generate` pass doesn't bury them in the auto-generated
// "harms way" comment block at the bottom of devicecloud.resolver.go.

import (
	"ironflyer/core/orchestrator/internal/operations/graph/model"
	"ironflyer/core/orchestrator/internal/operations/mobile/devicecloud"
)

func deviceCloudDeviceToGraphQL(d devicecloud.Device) model.DeviceCloudDevice {
	out := model.DeviceCloudDevice{
		ID:        d.ID,
		Provider:  string(d.Provider),
		Platform:  d.Platform,
		OsVersion: d.OSVersion,
		Model:     d.Model,
		Real:      d.Real,
	}
	if d.Manufacturer != "" {
		m := d.Manufacturer
		out.Manufacturer = &m
	}
	return out
}

func deviceCloudSessionToGraphQL(s *devicecloud.Session) *model.DeviceCloudSession {
	if s == nil {
		return nil
	}
	out := &model.DeviceCloudSession{
		ID:                  s.ID,
		Provider:            string(s.Provider),
		DeviceID:            s.DeviceID,
		Status:              s.Status,
		StartedAt:           s.StartedAt,
		ExpiresAt:           s.ExpiresAt,
		BillableMinutesUsed: s.BillableMinutesUsed,
	}
	if s.AppURL != "" {
		v := s.AppURL
		out.AppURL = &v
	}
	if s.SessionURL != "" {
		v := s.SessionURL
		out.SessionURL = &v
	}
	return out
}
