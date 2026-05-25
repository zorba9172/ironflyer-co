package pkg

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Compute is the set of outputs the data layer reads from the compute
// (ironflyer-infra) stack via pulumi.StackReference. Anything the compute
// stack does not yet export will be observed as an empty Output and the
// affected component will skip (with a Pulumi log) rather than crash.
type Compute struct {
	Ref                *pulumi.StackReference
	VpcID              pulumi.StringOutput
	PrivateSubnetIDs   pulumi.StringArrayOutput
	NodeSecurityGroup  pulumi.StringOutput
	ClusterSGID        pulumi.StringOutput
	DBSubnetGroupName  pulumi.StringOutput
	ClusterName        pulumi.StringOutput
	OidcProviderArn    pulumi.StringOutput
	OidcProviderURL    pulumi.StringOutput
	OrchestratorRole   pulumi.StringOutput
	RuntimeRole        pulumi.StringOutput
	BackupRole         pulumi.StringOutput
	HostedZoneName     pulumi.StringOutput
}

// LoadCompute resolves the compute stack outputs via StackReference. The
// caller passes the fully-qualified compute stack name (org/project/stack).
func LoadCompute(ctx *pulumi.Context, stackName string) (*Compute, error) {
	ref, err := pulumi.NewStackReference(ctx, stackName, nil)
	if err != nil {
		return nil, err
	}
	return &Compute{
		Ref:               ref,
		VpcID:             ref.GetStringOutput(pulumi.String("vpcId")),
		PrivateSubnetIDs:  ref.GetOutput(pulumi.String("privateSubnetIds")).ApplyT(toStringArray).(pulumi.StringArrayOutput),
		NodeSecurityGroup: ref.GetStringOutput(pulumi.String("nodeSecurityGroupId")),
		ClusterSGID:       ref.GetStringOutput(pulumi.String("clusterSecurityGroupId")),
		DBSubnetGroupName: ref.GetStringOutput(pulumi.String("dbSubnetGroupId")),
		ClusterName:       ref.GetStringOutput(pulumi.String("eksClusterName")),
		OidcProviderArn:   ref.GetStringOutput(pulumi.String("oidcProviderArn")),
		OidcProviderURL:   ref.GetStringOutput(pulumi.String("oidcProviderUrl")),
		OrchestratorRole:  ref.GetStringOutput(pulumi.String("orchestratorRoleArn")),
		RuntimeRole:       ref.GetStringOutput(pulumi.String("runtimeRoleArn")),
		BackupRole:        ref.GetStringOutput(pulumi.String("backupRoleArn")),
		HostedZoneName:    ref.GetStringOutput(pulumi.String("hostedZoneName")),
	}, nil
}

// toStringArray coerces an opaque interface{} stack-reference output into
// a []string. Pulumi returns []interface{} for array-typed exports.
func toStringArray(v interface{}) []string {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
