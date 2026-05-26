package appsec

import (
	"bytes"
	"strings"
	"time"

	cyclonedx "github.com/CycloneDX/cyclonedx-go"
	packageurl "github.com/package-url/packageurl-go"
)

func ToCycloneDX(projectID string, inv Inventory, generatedAt time.Time) *cyclonedx.BOM {
	bom := cyclonedx.NewBOM()
	bom.SpecVersion = cyclonedx.SpecVersion1_6
	bom.Version = 1
	projectName := firstNonEmpty(projectID, "user-project")
	bom.Metadata = &cyclonedx.Metadata{
		Timestamp: generatedAt.UTC().Format(time.RFC3339),
		Component: &cyclonedx.Component{
			Type:   cyclonedx.ComponentTypeApplication,
			Name:   projectName,
			BOMRef: "project:" + projectName,
		},
	}
	components := make([]cyclonedx.Component, 0, len(inv.Components))
	seen := map[string]bool{}
	for _, c := range inv.Components {
		purl := componentPURL(c)
		ref := purl
		if ref == "" {
			ref = c.Ecosystem + ":" + c.Name + "@" + c.Version
		}
		if seen[ref] {
			continue
		}
		seen[ref] = true
		scope := cyclonedx.ScopeRequired
		if c.Dev {
			scope = cyclonedx.ScopeOptional
		}
		props := []cyclonedx.Property{
			{Name: "ironflyer:ecosystem", Value: c.Ecosystem},
			{Name: "ironflyer:path", Value: c.Path},
		}
		if c.Deprecated != "" {
			props = append(props, cyclonedx.Property{Name: "ironflyer:deprecated", Value: c.Deprecated})
		}
		components = append(components, cyclonedx.Component{
			Type:       cyclonedx.ComponentTypeLibrary,
			Name:       c.Name,
			Version:    c.Version,
			BOMRef:     ref,
			PackageURL: purl,
			Scope:      scope,
			Properties: &props,
		})
	}
	bom.Components = &components
	return bom
}

func CycloneDXJSON(projectID string, inv Inventory, generatedAt time.Time) ([]byte, error) {
	var buf bytes.Buffer
	enc := cyclonedx.NewBOMEncoder(&buf, cyclonedx.BOMFileFormatJSON)
	enc.SetPretty(true)
	if err := enc.Encode(ToCycloneDX(projectID, inv, generatedAt)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func componentPURL(c Component) string {
	purlType := ""
	namespace := ""
	name := c.Name
	switch c.Ecosystem {
	case "npm":
		purlType = packageurl.TypeNPM
		if strings.HasPrefix(name, "@") {
			parts := strings.SplitN(name, "/", 2)
			if len(parts) == 2 {
				namespace = parts[0]
				name = parts[1]
			}
		}
	case "go":
		purlType = packageurl.TypeGolang
	case "pypi":
		purlType = packageurl.TypePyPi
	default:
		return ""
	}
	return packageurl.NewPackageURL(purlType, namespace, name, normaliseVersion(c.Version), nil, "").ToString()
}
