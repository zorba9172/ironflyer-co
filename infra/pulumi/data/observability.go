package data

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudwatch"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	helmv3 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// provisionObservability builds the cross-cutting telemetry surface:
//
//   - CloudWatch Log Groups for EKS control plane, RDS Postgres logs,
//     and the orchestrator / runtime / web application streams.
//   - kube-prometheus-stack (Prometheus + Grafana + AlertManager) into
//     `monitoring`.
//   - Optional loki-stack for log aggregation.
//   - Optional Datadog Agent if `infra:datadogApiKey` is set.
func provisionObservability(ctx *pulumi.Context, env *stackEnv, deps Compute) error {
	retention := env.logRetentionDays

	logGroups := []string{
		"/ironflyer/" + env.stack + "/app/orchestrator",
		"/ironflyer/" + env.stack + "/app/runtime",
		"/ironflyer/" + env.stack + "/app/web",
	}
	for _, lg := range logGroups {
		if _, err := cloudwatch.NewLogGroup(ctx, name(env, "log-"+slug(lg)), &cloudwatch.LogGroupArgs{
			Name:            pulumi.String(lg),
			RetentionInDays: pulumi.Int(retention),
			Tags:            env.tags,
		}); err != nil {
			return fmt.Errorf("log group %s: %w", lg, err)
		}
	}

	// EKS control plane logs land in /aws/eks/${cluster}/cluster. The
	// compute layer enables the log types on the cluster itself; we
	// just pre-create the group with our retention.
	if _, err := cloudwatch.NewLogGroup(ctx, name(env, "log-eks"), &cloudwatch.LogGroupArgs{
		Name: deps.EksClusterName.ApplyT(func(c string) string {
			return fmt.Sprintf("/aws/eks/%s/cluster", c)
		}).(pulumi.StringOutput),
		RetentionInDays: pulumi.Int(retention),
		Tags:            env.tags,
	}); err != nil {
		return fmt.Errorf("eks log group: %w", err)
	}

	if deps.K8sProvider == nil {
		// Without an authenticated K8s provider we stop at AWS-side
		// log groups. The compute agent will wire the kube provider in.
		return nil
	}

	opts := []pulumi.ResourceOption{pulumi.Provider(deps.K8sProvider)}

	monitoringNS, err := corev1.NewNamespace(ctx, name(env, "monitoring-ns"), &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("monitoring"),
			Labels: pulumi.StringMap{
				"app.kubernetes.io/part-of": pulumi.String("ironflyer-observability"),
			},
		},
	}, opts...)
	if err != nil {
		return fmt.Errorf("monitoring namespace: %w", err)
	}

	if _, err := helmv3.NewRelease(ctx, name(env, "kube-prometheus"), &helmv3.ReleaseArgs{
		Chart:           pulumi.String("kube-prometheus-stack"),
		Version:         pulumi.String("65.5.0"),
		Namespace:       monitoringNS.Metadata.Name().Elem(),
		CreateNamespace: pulumi.Bool(false),
		RepositoryOpts: &helmv3.RepositoryOptsArgs{
			Repo: pulumi.String("https://prometheus-community.github.io/helm-charts"),
		},
		Values: pulumi.Map{
			"grafana": pulumi.Map{
				"adminPassword":   pulumi.String("CHANGE-ME-VIA-SECRETS"),
				"defaultDashboardsTimezone": pulumi.String("UTC"),
			},
			"prometheus": pulumi.Map{
				"prometheusSpec": pulumi.Map{
					"retention": pulumi.String("15d"),
					"storageSpec": pulumi.Map{
						"volumeClaimTemplate": pulumi.Map{
							"spec": pulumi.Map{
								"storageClassName": pulumi.String("gp3"),
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
	}, opts...); err != nil {
		return fmt.Errorf("kube-prometheus-stack: %w", err)
	}

	// Loki is optional but cheap to wire and the orchestrator agents
	// know how to push to it when present.
	if _, err := helmv3.NewRelease(ctx, name(env, "loki-stack"), &helmv3.ReleaseArgs{
		Chart:           pulumi.String("loki-stack"),
		Version:         pulumi.String("2.10.2"),
		Namespace:       monitoringNS.Metadata.Name().Elem(),
		CreateNamespace: pulumi.Bool(false),
		RepositoryOpts: &helmv3.RepositoryOptsArgs{
			Repo: pulumi.String("https://grafana.github.io/helm-charts"),
		},
		Values: pulumi.Map{
			"loki":     pulumi.Map{"persistence": pulumi.Map{"enabled": pulumi.Bool(true), "storageClassName": pulumi.String("gp3"), "size": pulumi.String("20Gi")}},
			"promtail": pulumi.Map{"enabled": pulumi.Bool(true)},
		},
	}, opts...); err != nil {
		return fmt.Errorf("loki-stack: %w", err)
	}

	if env.datadogApiKey != "" {
		if _, err := helmv3.NewRelease(ctx, name(env, "datadog"), &helmv3.ReleaseArgs{
			Chart:           pulumi.String("datadog"),
			Version:         pulumi.String("3.78.0"),
			Namespace:       monitoringNS.Metadata.Name().Elem(),
			CreateNamespace: pulumi.Bool(false),
			RepositoryOpts: &helmv3.RepositoryOptsArgs{
				Repo: pulumi.String("https://helm.datadoghq.com"),
			},
			Values: pulumi.Map{
				"datadog": pulumi.Map{
					"apiKey":     pulumi.String(env.datadogApiKey),
					"site":       pulumi.String("datadoghq.com"),
					"clusterName": pulumi.String("ironflyer-" + env.stack),
					"logs": pulumi.Map{
						"enabled":             pulumi.Bool(true),
						"containerCollectAll": pulumi.Bool(true),
					},
					"apm": pulumi.Map{
						"portEnabled": pulumi.Bool(true),
					},
				},
			},
		}, opts...); err != nil {
			return fmt.Errorf("datadog agent: %w", err)
		}
	}

	return nil
}

// slug normalizes a CloudWatch log-group path into a Pulumi-safe alias.
func slug(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			b = append(b, c)
		default:
			if len(b) > 0 && b[len(b)-1] != '-' {
				b = append(b, '-')
			}
		}
	}
	for len(b) > 0 && b[len(b)-1] == '-' {
		b = b[:len(b)-1]
	}
	return string(b)
}
