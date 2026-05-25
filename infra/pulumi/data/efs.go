package data

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/efs"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// EFS is the shared file system the runtime pool mounts so workspace
// state survives pod reschedules. The runtime DaemonSet / StatefulSet
// mounts the access point at /workspaces with uid/gid 1000 (the runtime
// container's non-root user).
type EFS struct {
	FileSystem    *efs.FileSystem
	AccessPoint   *efs.AccessPoint
	FileSystemID  pulumi.StringOutput
	AccessPointID pulumi.StringOutput
	DnsName       pulumi.StringOutput
}

func provisionEFS(ctx *pulumi.Context, env *stackEnv, deps Compute, k *KMSKeys, dataSG *ec2.SecurityGroup) (*EFS, error) {
	// NFS (2049) from the EKS cluster SG to the data SG.
	if _, err := ec2.NewSecurityGroupRule(ctx, name(env, "data-sg-ingress-nfs"), &ec2.SecurityGroupRuleArgs{
		Type:                  pulumi.String("ingress"),
		FromPort:              pulumi.Int(2049),
		ToPort:                pulumi.Int(2049),
		Protocol:              pulumi.String("tcp"),
		SecurityGroupId:       dataSG.ID(),
		SourceSecurityGroupId: deps.ClusterSGID,
		Description:           pulumi.String("EFS NFS from EKS cluster SG"),
	}); err != nil {
		return nil, err
	}

	fs, err := efs.NewFileSystem(ctx, name(env, "efs"), &efs.FileSystemArgs{
		CreationToken:   pulumi.String(name(env, "efs")),
		Encrypted:       pulumi.Bool(true),
		KmsKeyId:        k.EBSKey.Arn,
		PerformanceMode: pulumi.String("generalPurpose"),
		ThroughputMode:  pulumi.String("elastic"),
		Tags:            env.tags,
	})
	if err != nil {
		return nil, err
	}

	// One mount target per private subnet — runtime pods land in any AZ.
	deps.PrivateSubnetIDs.ApplyT(func(ids []string) ([]string, error) {
		for i, sid := range ids {
			if _, err := efs.NewMountTarget(ctx, name(env, "efs-mt-")+itoa(i), &efs.MountTargetArgs{
				FileSystemId:   fs.ID(),
				SubnetId:       pulumi.String(sid),
				SecurityGroups: pulumi.StringArray{dataSG.ID()},
			}); err != nil {
				return nil, err
			}
		}
		return ids, nil
	})

	ap, err := efs.NewAccessPoint(ctx, name(env, "efs-ap-workspaces"), &efs.AccessPointArgs{
		FileSystemId: fs.ID(),
		PosixUser: &efs.AccessPointPosixUserArgs{
			Uid: pulumi.Int(1000),
			Gid: pulumi.Int(1000),
		},
		RootDirectory: &efs.AccessPointRootDirectoryArgs{
			Path: pulumi.String("/workspaces"),
			CreationInfo: &efs.AccessPointRootDirectoryCreationInfoArgs{
				OwnerUid:    pulumi.Int(1000),
				OwnerGid:    pulumi.Int(1000),
				Permissions: pulumi.String("0775"),
			},
		},
		Tags: env.tags,
	})
	if err != nil {
		return nil, err
	}

	return &EFS{
		FileSystem:    fs,
		AccessPoint:   ap,
		FileSystemID:  fs.ID().ToStringOutput(),
		AccessPointID: ap.ID().ToStringOutput(),
		DnsName:       fs.DnsName,
	}, nil
}

// itoa avoids pulling in strconv just for a Pulumi resource alias.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
