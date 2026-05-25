// Ironflyer compute + data + edge infrastructure (DigitalOcean).
//
// This program is the DO counterpart to the AWS Pulumi project at
// `infra/pulumi/`. They coexist — the operator picks whichever cloud
// they want by running `pulumi up` in the corresponding directory.
//
// Split:
//   * compute/  — VPC + DOKS + node pools + the built-in autoscaler +
//                 the kubernetes.Provider downstream layers reuse.
//   * data/     — Managed Postgres + Redis + Spaces (sibling agent).
//   * edge/     — Cloudflare DNS + cert-manager + sealed-secrets +
//                 ingress-nginx + Vercel (sibling agent).
//
// The data + edge packages are stubbed today so the program builds
// end-to-end while the sibling agents fill in the bodies. The Inputs /
// Outputs structs are the locked hand-off contracts — change them
// together, never one-sided.
package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"ironflyer/infra/pulumi-do/compute"
	"ironflyer/infra/pulumi-do/data"
	"ironflyer/infra/pulumi-do/edge"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg, err := compute.LoadConfig(ctx)
		if err != nil {
			return err
		}

		// --- VPC ---------------------------------------------------------
		net, err := compute.NewNetwork(ctx, cfg)
		if err != nil {
			return err
		}

		// --- DOKS (control plane + system pool + runtime pool) -----------
		cluster, err := compute.NewDOKS(ctx, cfg, net)
		if err != nil {
			return err
		}

		// --- kubernetes.Provider authenticated against the DOKS cluster --
		k8sProvider, err := compute.NewK8sProvider(ctx, cfg, cluster)
		if err != nil {
			return err
		}

		// --- Data layer (managed Postgres, Redis, Spaces) ----------------
		dataOut, err := data.Provision(ctx, data.Inputs{
			Config:      cfg,
			Network:     net,
			Cluster:     cluster,
			K8sProvider: k8sProvider,
		})
		if err != nil {
			return err
		}

		// --- Edge layer (Cloudflare + ingress + Vercel) ------------------
		if err := edge.Provision(ctx, edge.Inputs{
			Config:      cfg,
			Network:     net,
			Cluster:     cluster,
			K8sProvider: k8sProvider,
			Data:        dataOut,
		}); err != nil {
			return err
		}

		// --- Compute exports (consumed by `pulumi stack output` + by the
		//     sibling Pulumi projects if they ever read this stack by
		//     StackReference). Kubeconfig is wrapped in ToSecret so it
		//     never lands in plain-text state diffs. ---------------------
		ctx.Export("region", pulumi.String(cfg.Region))
		ctx.Export("vpcId", net.VpcID)
		ctx.Export("vpcIpRange", net.IPRange)
		ctx.Export("doksClusterId", cluster.ID())
		ctx.Export("doksClusterName", cluster.Name)
		ctx.Export("doksEndpoint", cluster.Endpoint)
		ctx.Export("doksVersion", cluster.Version)
		ctx.Export("doksKubeconfig", pulumi.ToSecret(cluster.KubeConfigs.Index(pulumi.Int(0)).RawConfig().Elem()))

		// --- Data exports (zero-valued until the data agent lands; the
		//     export keys are stable so consumers can read them
		//     unconditionally). ----------------------------------------
		ctx.Export("postgresHost", dataOut.PostgresHost)
		ctx.Export("postgresPort", dataOut.PostgresPort)
		ctx.Export("postgresDatabase", dataOut.PostgresDatabase)
		ctx.Export("postgresUser", dataOut.PostgresUser)
		ctx.Export("postgresConnectionUri", pulumi.ToSecret(dataOut.PostgresConnectionURI))
		ctx.Export("redisHost", dataOut.RedisHost)
		ctx.Export("redisPort", dataOut.RedisPort)
		ctx.Export("redisConnectionUri", pulumi.ToSecret(dataOut.RedisConnectionURI))
		ctx.Export("spacesEndpoint", dataOut.SpacesEndpoint)
		ctx.Export("spacesBucket", dataOut.SpacesBucket)
		ctx.Export("spacesRegion", dataOut.SpacesRegion)

		return nil
	})
}
