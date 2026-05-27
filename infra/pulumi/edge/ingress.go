package edge

import (
	"fmt"

	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	helmv3 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// newIngressNginx installs the kubernetes/ingress-nginx controller and
// returns (a) the public LoadBalancer IP it advertises (for Cloudflare
// DNS records), and (b) the underlying Helm release (so the ClusterIssuer
// can wait on it via DependsOn).
//
// All of the DO-specific behavior lives in the LB service annotations:
//
//   - do-loadbalancer-name           — readable LB name in the DO console.
//   - do-loadbalancer-protocol=https — terminate TLS upstream of the LB
//     so we keep proxy-protocol headers without unwrapping cert chains in
//     the DO LB itself.
//   - do-loadbalancer-tls-passthrough=true — DO passes the bytes through
//     to ingress-nginx, which terminates TLS using cert-manager-issued
//     certs. This is critical: it lets us serve real Let's Encrypt certs
//     for `api.<root>` end-to-end (the LB does not re-sign).
//   - do-loadbalancer-redirect-http-to-https=true — DO does the redirect
//     at the LB so the controller never sees plain :80 traffic.
//   - do-loadbalancer-enable-proxy-protocol=true — preserves client IP
//     through the LB, paired with `use-proxy-protocol=true` on the
//     controller config map so nginx interprets it.
//
// Autoscaling + PDB defaults follow the kubernetes/ingress-nginx
// production recommendation: min 2 replicas, max 6, 70% CPU target,
// at-least-one-available PDB.
func newIngressNginx(ctx *pulumi.Context, in Inputs, cmRel, ssRel *helmv3.Release) (pulumi.StringOutput, *helmv3.Release, error) {
	cfg := in.Config
	rel, err := helmv3.NewRelease(ctx, "ingress-nginx", &helmv3.ReleaseArgs{
		Chart:           pulumi.String("ingress-nginx"),
		Version:         pulumi.String("4.11.3"),
		Namespace:       pulumi.String("ingress-nginx"),
		CreateNamespace: pulumi.Bool(true),
		RepositoryOpts: &helmv3.RepositoryOptsArgs{
			Repo: pulumi.String("https://kubernetes.github.io/ingress-nginx"),
		},
		Values: pulumi.Map{
			"controller": pulumi.Map{
				"ingressClassResource": pulumi.Map{
					"name":            pulumi.String("nginx"),
					"enabled":         pulumi.Bool(true),
					"default":         pulumi.Bool(true),
					"controllerValue": pulumi.String("k8s.io/ingress-nginx"),
				},
				"service": pulumi.Map{
					"type": pulumi.String("LoadBalancer"),
					// DigitalOcean LB annotations. Strings only — DO's
					// in-tree cloud controller parses them.
					"annotations": pulumi.Map{
						"service.beta.kubernetes.io/do-loadbalancer-name":                       pulumi.String("ironflyer-" + cfg.Stack),
						"service.beta.kubernetes.io/do-loadbalancer-protocol":                   pulumi.String("https"),
						"service.beta.kubernetes.io/do-loadbalancer-tls-passthrough":            pulumi.String("true"),
						"service.beta.kubernetes.io/do-loadbalancer-redirect-http-to-https":     pulumi.String("true"),
						"service.beta.kubernetes.io/do-loadbalancer-enable-proxy-protocol":      pulumi.String("true"),
						"service.beta.kubernetes.io/do-loadbalancer-enable-backend-keepalive":   pulumi.String("true"),
						"service.beta.kubernetes.io/do-loadbalancer-healthcheck-protocol":       pulumi.String("http"),
						"service.beta.kubernetes.io/do-loadbalancer-healthcheck-port":           pulumi.String("10254"),
						"service.beta.kubernetes.io/do-loadbalancer-healthcheck-path":           pulumi.String("/healthz"),
						"service.beta.kubernetes.io/do-loadbalancer-disable-lets-encrypt-dns-records": pulumi.String("true"),
					},
				},
				"config": pulumi.Map{
					// Required for client-IP preservation through the DO LB.
					"use-proxy-protocol":           pulumi.String("true"),
					"use-forwarded-headers":        pulumi.String("true"),
					"compute-full-forwarded-for":   pulumi.String("true"),
					"enable-real-ip":               pulumi.String("true"),
					"proxy-body-size":              pulumi.String("32m"),
					"proxy-read-timeout":           pulumi.String("3600"),
					"proxy-send-timeout":           pulumi.String("3600"),
					"ssl-redirect":                 pulumi.String("true"),
					"hsts":                         pulumi.String("true"),
					"hsts-include-subdomains":      pulumi.String("true"),
					"hsts-preload":                 pulumi.String("true"),
				},
				"metrics": pulumi.Map{
					"enabled": pulumi.Bool(true),
				},
				"podDisruptionBudget": pulumi.Map{
					"enabled":      pulumi.Bool(true),
					"minAvailable": pulumi.Int(1),
				},
				"autoscaling": pulumi.Map{
					"enabled":                        pulumi.Bool(true),
					"minReplicas":                    pulumi.Int(2),
					"maxReplicas":                    pulumi.Int(6),
					"targetCPUUtilizationPercentage": pulumi.Int(70),
				},
				// Always-prefer the system node pool. Runtime nodes are
				// tainted; ingress should never schedule there.
				"nodeSelector": pulumi.Map{
					"workload": pulumi.String("system"),
				},
				"affinity": pulumi.Map{
					"podAntiAffinity": pulumi.Map{
						"preferredDuringSchedulingIgnoredDuringExecution": pulumi.MapArray{
							pulumi.Map{
								"weight": pulumi.Int(100),
								"podAffinityTerm": pulumi.Map{
									"topologyKey": pulumi.String("kubernetes.io/hostname"),
									"labelSelector": pulumi.Map{
										"matchLabels": pulumi.Map{
											"app.kubernetes.io/name":      pulumi.String("ingress-nginx"),
											"app.kubernetes.io/component": pulumi.String("controller"),
										},
									},
								},
							},
						},
					},
				},
			},
			"defaultBackend": pulumi.Map{
				"enabled": pulumi.Bool(false),
			},
		},
	},
		pulumi.Provider(in.K8sProvider),
		// Sealed-secrets being up before ingress isn't strictly required
		// but keeps the ordering deterministic across redeploys.
		pulumi.DependsOn([]pulumi.Resource{cmRel, ssRel}),
	)
	if err != nil {
		return pulumi.StringOutput{}, nil, err
	}

	// Reach back into the cluster for the controller's Service so we can
	// pull the LoadBalancer IP. The Helm release does not surface this
	// directly — DigitalOcean's cloud controller patches the Service's
	// `status.loadBalancer.ingress[0].ip` after the LB is healthy, and
	// pulumi-kubernetes waits on that before resolving.
	//
	// The GetService call runs inside an ApplyT so we can build the
	// `<namespace>/<release-name>-controller` ID from the Helm release
	// outputs (only known after Helm runs). Returning a StringOutput
	// from ApplyT auto-flattens — Pulumi unwraps the nested Output so
	// callers get a single StringOutput.
	lbIP := pulumi.All(rel.Status.Namespace(), rel.Status.Name()).ApplyT(func(args []any) (pulumi.StringOutput, error) {
		ns := ""
		if p, ok := args[0].(*string); ok && p != nil {
			ns = *p
		}
		name := ""
		if p, ok := args[1].(*string); ok && p != nil {
			name = *p
		}
		svc, err := corev1.GetService(ctx,
			"ingress-nginx-controller-svc",
			pulumi.ID(fmt.Sprintf("%s/%s-controller", ns, name)),
			nil,
			pulumi.Provider(in.K8sProvider),
			pulumi.DependsOn([]pulumi.Resource{rel}),
		)
		if err != nil {
			return pulumi.StringOutput{}, err
		}
		return svc.Status.LoadBalancer().Ingress().Index(pulumi.Int(0)).Ip().Elem(), nil
	}).(pulumi.StringOutput)
	return lbIP, rel, nil
}
