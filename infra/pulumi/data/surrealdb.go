package data

import (
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Surreal is the in-cluster SurrealDB deployment. We do this as a hand-
// rolled StatefulSet rather than the official chart because:
//
//  1. The chart doesn't yet support raft cluster mode out of the box.
//  2. We need a deterministic Service DNS name that the orchestrator
//     hardcodes (`surreal.ironflyer.svc.cluster.local:8000`).
//  3. The PVC + storage class needs to match the EBS gp3 storage class
//     the compute agent provisions for the cluster.
//
// In dev stacks (or when infra:surrealEnabled=false) this is skipped
// entirely; the orchestrator still has Postgres as its primary store.
type Surreal struct {
	Service pulumi.StringOutput
}

func provisionSurreal(ctx *pulumi.Context, env *stackEnv, deps Compute, secrets *Secrets) (*Surreal, error) {
	if !env.surrealEnabled || deps.K8sProvider == nil {
		return &Surreal{
			Service: pulumi.String("").ToStringOutput(),
		}, nil
	}

	opts := []pulumi.ResourceOption{pulumi.Provider(deps.K8sProvider)}

	ns, err := corev1.NewNamespace(ctx, name(env, "surreal-ns"), &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("ironflyer"),
			Labels: pulumi.StringMap{
				"app.kubernetes.io/part-of": pulumi.String("ironflyer"),
				"ironflyer.io/layer":        pulumi.String("data"),
			},
		},
	}, opts...)
	if err != nil {
		return nil, err
	}

	labels := pulumi.StringMap{
		"app.kubernetes.io/name":     pulumi.String("surrealdb"),
		"app.kubernetes.io/part-of":  pulumi.String("ironflyer"),
		"app.kubernetes.io/instance": pulumi.String("ironflyer-" + env.stack),
	}

	// Headless service for the StatefulSet (raft peer discovery).
	headless, err := corev1.NewService(ctx, name(env, "surreal-headless"), &corev1.ServiceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("surreal-headless"),
			Namespace: ns.Metadata.Name(),
			Labels:    labels,
		},
		Spec: &corev1.ServiceSpecArgs{
			ClusterIP: pulumi.String("None"),
			Selector:  labels,
			Ports: corev1.ServicePortArray{
				&corev1.ServicePortArgs{Name: pulumi.String("http"), Port: pulumi.Int(8000), TargetPort: pulumi.Int(8000)},
			},
		},
	}, opts...)
	if err != nil {
		return nil, err
	}

	// Stable client-facing service: surreal.ironflyer.svc.cluster.local:8000
	svc, err := corev1.NewService(ctx, name(env, "surreal-svc"), &corev1.ServiceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("surreal"),
			Namespace: ns.Metadata.Name(),
			Labels:    labels,
		},
		Spec: &corev1.ServiceSpecArgs{
			Type:     pulumi.String("ClusterIP"),
			Selector: labels,
			Ports: corev1.ServicePortArray{
				&corev1.ServicePortArgs{Name: pulumi.String("http"), Port: pulumi.Int(8000), TargetPort: pulumi.Int(8000)},
			},
		},
	}, opts...)
	if err != nil {
		return nil, err
	}

	replicas := 3
	if !env.isProd {
		replicas = 1
	}

	if _, err := appsv1.NewStatefulSet(ctx, name(env, "surreal-sts"), &appsv1.StatefulSetArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("surreal"),
			Namespace: ns.Metadata.Name(),
			Labels:    labels,
		},
		Spec: &appsv1.StatefulSetSpecArgs{
			ServiceName: headless.Metadata.Name().Elem(),
			Replicas:    pulumi.Int(replicas),
			Selector:    &metav1.LabelSelectorArgs{MatchLabels: labels},
			Template: &corev1.PodTemplateSpecArgs{
				Metadata: &metav1.ObjectMetaArgs{Labels: labels},
				Spec: &corev1.PodSpecArgs{
					ServiceAccountName: pulumi.String("orchestrator-sa"),
					SecurityContext: &corev1.PodSecurityContextArgs{
						FsGroup:      pulumi.Int(1000),
						RunAsUser:    pulumi.Int(1000),
						RunAsNonRoot: pulumi.Bool(true),
					},
					Containers: corev1.ContainerArray{
						&corev1.ContainerArgs{
							Name:  pulumi.String("surreal"),
							Image: pulumi.String("surrealdb/surrealdb:v2.1.4"),
							Args: pulumi.StringArray{
								pulumi.String("start"),
								pulumi.String("--bind"), pulumi.String("0.0.0.0:8000"),
								pulumi.String("--user"), pulumi.String("$(SURREAL_USER)"),
								pulumi.String("--pass"), pulumi.String("$(SURREAL_PASS)"),
								pulumi.String("file:/data/ironflyer.db"),
							},
							Env: corev1.EnvVarArray{
								&corev1.EnvVarArgs{
									Name: pulumi.String("SURREAL_USER"),
									ValueFrom: &corev1.EnvVarSourceArgs{
										SecretKeyRef: &corev1.SecretKeySelectorArgs{
											Name: pulumi.String("ironflyer-surreal-root"),
											Key:  pulumi.String("username"),
										},
									},
								},
								&corev1.EnvVarArgs{
									Name: pulumi.String("SURREAL_PASS"),
									ValueFrom: &corev1.EnvVarSourceArgs{
										SecretKeyRef: &corev1.SecretKeySelectorArgs{
											Name: pulumi.String("ironflyer-surreal-root"),
											Key:  pulumi.String("password"),
										},
									},
								},
							},
							Ports: corev1.ContainerPortArray{
								&corev1.ContainerPortArgs{Name: pulumi.String("http"), ContainerPort: pulumi.Int(8000)},
							},
							VolumeMounts: corev1.VolumeMountArray{
								&corev1.VolumeMountArgs{Name: pulumi.String("data"), MountPath: pulumi.String("/data")},
							},
							ReadinessProbe: &corev1.ProbeArgs{
								HttpGet: &corev1.HTTPGetActionArgs{
									Path: pulumi.String("/health"),
									Port: pulumi.Int(8000),
								},
								InitialDelaySeconds: pulumi.Int(10),
								PeriodSeconds:       pulumi.Int(10),
							},
						},
					},
				},
			},
			VolumeClaimTemplates: corev1.PersistentVolumeClaimTypeArray{
				&corev1.PersistentVolumeClaimTypeArgs{
					Metadata: &metav1.ObjectMetaArgs{Name: pulumi.String("data")},
					Spec: &corev1.PersistentVolumeClaimSpecArgs{
						AccessModes:      pulumi.StringArray{pulumi.String("ReadWriteOnce")},
						StorageClassName: pulumi.String("gp3"),
						Resources: &corev1.VolumeResourceRequirementsArgs{
							Requests: pulumi.StringMap{"storage": pulumi.String("50Gi")},
						},
					},
				},
			},
		},
	}, opts...); err != nil {
		return nil, err
	}

	_ = secrets // referenced via ExternalSecret in consumers.go.

	return &Surreal{
		Service: pulumi.All(svc.Metadata.Name(), svc.Metadata.Namespace()).ApplyT(func(args []any) string {
			n, _ := args[0].(*string)
			ns, _ := args[1].(*string)
			if n == nil || ns == nil {
				return ""
			}
			return *n + "." + *ns + ".svc.cluster.local:8000"
		}).(pulumi.StringOutput),
	}, nil
}
