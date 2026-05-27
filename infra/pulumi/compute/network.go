package compute

import (
	"github.com/pulumi/pulumi-digitalocean/sdk/v4/go/digitalocean"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Network bundles every DO networking output downstream modules need.
// DOKS, managed databases, and the edge layer all attach to the same VPC
// so private traffic stays off the public internet (DO managed databases
// support trusted-source VPCs natively).
type Network struct {
	VpcID   pulumi.IDOutput
	VpcURN  pulumi.StringOutput
	IPRange pulumi.StringOutput
	Region  pulumi.StringOutput
}

// NewNetwork provisions the per-stack DO VPC. DigitalOcean handles
// subnetting + routing internally — there's no public/private subnet split
// to manage like AWS, which keeps this file refreshingly small.
func NewNetwork(ctx *pulumi.Context, cfg *Config) (*Network, error) {
	name := cfg.ResourceName("vpc")
	vpc, err := digitalocean.NewVpc(ctx, name, &digitalocean.VpcArgs{
		Name:        pulumi.String(name),
		Region:      pulumi.String(cfg.Region),
		IpRange:     pulumi.String("10.10.0.0/16"),
		Description: pulumi.String("Ironflyer VPC — DOKS, managed Postgres, managed Redis attach here."),
	})
	if err != nil {
		return nil, err
	}

	return &Network{
		VpcID:   vpc.ID(),
		VpcURN:  vpc.VpcUrn,
		IPRange: vpc.IpRange,
		Region:  vpc.Region,
	}, nil
}
