package data

import (
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	helmv3 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// provisionObservability installs kube-prometheus-stack + loki-stack
// into a `monitoring` namespace. Gated on `ironflyer:observabilityEnabled`
// (defaults to true for stacks whose name begins with `prod`, false
// otherwise). DigitalOcean has no managed CloudWatch analogue, so all
// telemetry runs in-cluster — the orchestrator already ships Prometheus
// metrics + structured JSON logs that Loki ingests via Promtail.
//
// Skipped entirely when the kubernetes provider isn't wired in yet.
func provisionObservability(ctx *pulumi.Context, in Inputs) error {
	if in.K8sProvider == nil {
		return nil
	}

	cfg := in.Config
	defaultEnabled := isProd(cfg.Stack)
	c := config.New(ctx, "ironflyer")
	enabled := defaultEnabled
	if v, err := c.TryBool("observabilityEnabled"); err == nil {
		enabled = v
	}
	if !enabled {
		return nil
	}

	opts := []pulumi.ResourceOption{pulumi.Provider(in.K8sProvider)}

	ns, err := corev1.NewNamespace(ctx, cfg.ResourceName("monitoring-ns"), &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("monitoring"),
			Labels: pulumi.StringMap{
				"app.kubernetes.io/part-of": pulumi.String("ironflyer-observability"),
				"ironflyer.dev/stack":       pulumi.String(cfg.Stack),
			},
		},
	}, opts...)
	if err != nil {
		return err
	}

	releaseOpts := append([]pulumi.ResourceOption{}, opts...)
	releaseOpts = append(releaseOpts, pulumi.DependsOn([]pulumi.Resource{ns}))

	// kube-prometheus-stack: Prometheus + Grafana + Alertmanager. The
	// chart's defaults are sane; we override the Prometheus retention +
	// storage class so the DOKS-supplied `do-block-storage` provisioner
	// claims block volumes instead of trying to use AWS gp3 (which the
	// AWS-side reference values use).
	if _, err := helmv3.NewRelease(ctx, cfg.ResourceName("kube-prometheus"), &helmv3.ReleaseArgs{
		Chart:           pulumi.String("kube-prometheus-stack"),
		Version:         pulumi.String("65.5.0"),
		Namespace:       ns.Metadata.Name().Elem(),
		CreateNamespace: pulumi.Bool(false),
		RepositoryOpts: &helmv3.RepositoryOptsArgs{
			Repo: pulumi.String("https://prometheus-community.github.io/helm-charts"),
		},
		Values: pulumi.Map{
			"grafana": pulumi.Map{
				"adminPassword":             pulumi.String("CHANGE-ME-VIA-SECRETS"),
				"defaultDashboardsTimezone": pulumi.String("UTC"),
				"persistence": pulumi.Map{
					"enabled":          pulumi.Bool(true),
					"storageClassName": pulumi.String("do-block-storage"),
					"size":             pulumi.String("10Gi"),
				},
			},
			"prometheus": pulumi.Map{
				"prometheusSpec": pulumi.Map{
					"retention": pulumi.String("15d"),
					"storageSpec": pulumi.Map{
						"volumeClaimTemplate": pulumi.Map{
							"spec": pulumi.Map{
								"storageClassName": pulumi.String("do-block-storage"),
								"accessModes":      pulumi.Array{pulumi.String("ReadWriteOnce")},
								"resources": pulumi.Map{
									"requests": pulumi.Map{"storage": pulumi.String("50Gi")},
								},
							},
						},
					},
				},
			},
		},
	}, releaseOpts...); err != nil {
		return err
	}

	if _, err := helmv3.NewRelease(ctx, cfg.ResourceName("loki-stack"), &helmv3.ReleaseArgs{
		Chart:           pulumi.String("loki-stack"),
		Version:         pulumi.String("2.10.2"),
		Namespace:       ns.Metadata.Name().Elem(),
		CreateNamespace: pulumi.Bool(false),
		RepositoryOpts: &helmv3.RepositoryOptsArgs{
			Repo: pulumi.String("https://grafana.github.io/helm-charts"),
		},
		Values: pulumi.Map{
			"loki": pulumi.Map{
				"persistence": pulumi.Map{
					"enabled":          pulumi.Bool(true),
					"storageClassName": pulumi.String("do-block-storage"),
					"size":             pulumi.String("20Gi"),
				},
			},
			"promtail": pulumi.Map{"enabled": pulumi.Bool(true)},
		},
	}, releaseOpts...); err != nil {
		return err
	}

	return nil
}

func isProd(stack string) bool {
	return len(stack) >= 4 && stack[:4] == "prod"
}
