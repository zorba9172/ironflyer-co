package eas

import (
	"context"
	"errors"
	"strings"
)

// PublishOTAUpdate is the thin wrapper around PublishUpdate the
// orchestrator's mobilePublishUpdate resolver calls. Splits the
// PublishUpdateRequest into positional args so the resolver layer
// stays free of the eas package's struct literal noise.
func (c *Client) PublishOTAUpdate(
	ctx context.Context,
	channelName string,
	branch string,
	message string,
	runtimeVersion string,
	extra map[string]any,
) (*Update, error) {
	if strings.TrimSpace(channelName) == "" {
		return nil, errors.New("eas: PublishOTAUpdate: empty channelName")
	}
	if strings.TrimSpace(runtimeVersion) == "" {
		return nil, errors.New("eas: PublishOTAUpdate: empty runtimeVersion")
	}
	if strings.TrimSpace(branch) == "" {
		branch = "main"
	}
	return c.PublishUpdate(ctx, channelName, PublishUpdateRequest{
		Branch:         branch,
		Message:        message,
		RuntimeVersion: runtimeVersion,
		ManifestExtra:  extra,
	})
}
