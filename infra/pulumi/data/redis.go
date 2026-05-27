package data

import (
	"github.com/pulumi/pulumi-digitalocean/sdk/v4/go/digitalocean"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Redis is the managed DO Valkey (Redis-compatible) cluster + outputs.
//
// DigitalOcean transitioned Redis-branded managed clusters to Valkey;
// the engine slug is `valkey`, Redis clients continue to work unchanged
// (Valkey is a drop-in protocol-compatible fork).
type Redis struct {
	Cluster     *digitalocean.DatabaseCluster
	Host        pulumi.StringOutput
	Port        pulumi.IntOutput
	PrivateHost pulumi.StringOutput
	URI         pulumi.StringOutput // secret
	PrivateURI  pulumi.StringOutput // secret
	Password    pulumi.StringOutput // secret
}

func provisionRedis(ctx *pulumi.Context, in Inputs) (*Redis, error) {
	cfg := in.Config
	clusterName := cfg.ResourceName("redis")

	cluster, err := digitalocean.NewDatabaseCluster(ctx, clusterName, &digitalocean.DatabaseClusterArgs{
		Name:               pulumi.String(clusterName),
		Engine:             pulumi.String("valkey"),
		Version:            pulumi.String(cfg.RedisVersion),
		Size:               pulumi.String(cfg.RedisSize),
		Region:             pulumi.String(cfg.Region),
		NodeCount:          pulumi.Int(IfHA(cfg, 2, 1)),
		PrivateNetworkUuid: in.Network.VpcID.ToStringOutput(),
		EvictionPolicy:     pulumi.String("allkeys_lru"),
		Tags:               cfg.Tags("data", "redis", "valkey"),
		MaintenanceWindows: digitalocean.DatabaseClusterMaintenanceWindowArray{
			&digitalocean.DatabaseClusterMaintenanceWindowArgs{
				Day:  pulumi.String("sunday"),
				Hour: pulumi.String("04:00:00"),
			},
		},
	})
	if err != nil {
		return nil, err
	}

	// Trusted-source firewall identical pattern to Postgres: only the
	// DOKS cluster's worker droplets can dial the Valkey endpoint.
	if _, err := digitalocean.NewDatabaseFirewall(ctx, cfg.ResourceName("redis-fw"), &digitalocean.DatabaseFirewallArgs{
		ClusterId: cluster.ID(),
		Rules: digitalocean.DatabaseFirewallRuleArray{
			&digitalocean.DatabaseFirewallRuleArgs{
				Type:  pulumi.String("k8s"),
				Value: in.Cluster.ID().ToStringOutput(),
			},
		},
	}); err != nil {
		return nil, err
	}

	return &Redis{
		Cluster:     cluster,
		Host:        cluster.Host,
		Port:        cluster.Port,
		PrivateHost: cluster.PrivateHost,
		URI:         cluster.Uri,
		PrivateURI:  cluster.PrivateUri,
		Password:    cluster.Password,
	}, nil
}
