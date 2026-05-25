// Package pkg holds the shared env + naming helpers used by the data
// layer's component subpackages. Keeping it under pkg/ (versus a sibling
// package) avoids an import cycle: postgres, redis, s3, secrets, kms, and
// k8s all depend on the same stack-derived configuration.
package pkg

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// Env captures the per-stack knobs every component needs.
type Env struct {
	Stack                       string
	Region                      string
	ReplicaRegion               string
	IsProd                      bool
	IsDev                       bool
	DBName                      string
	DBUser                      string
	PostgresEngineVersion       string
	RedisEngineVersion          string
	BackupWindow                string
	MaintenanceWindow           string
	ESOInstall                  bool
	ESONamespace                string
	AppNamespace                string
	Kubeconfig                  string
	CrossRegionReplication      bool
	ComputeStackName            string
	BackupRetentionDays         int
	KMSDeletionWindowDays       int
	PostgresReaders             int
	PostgresProvisioned         bool
	PostgresMinACU              float64
	PostgresMaxACU              float64
	PostgresInstanceClass       string
	RedisShards                 int
	RedisReplicasPerShard       int
	RedisSnapshotRetentionDays  int
	Tags                        pulumi.StringMap
}

// Resolve walks the active stack config and assembles an Env.
func Resolve(ctx *pulumi.Context) (*Env, error) {
	stack := ctx.Stack()
	awsCfg := config.New(ctx, "aws")
	region := awsCfg.Require("region")
	dataCfg := config.New(ctx, "data")

	isProd := strings.HasPrefix(stack, "prod")
	isDev := stack == "dev"

	computeStack := dataCfg.Get("computeStack")
	if computeStack == "" {
		computeStack = fmt.Sprintf("ironflyer-infra/%s", stack)
	}

	esoInstall := true
	if v, err := dataCfg.TryBool("eso.install"); err == nil {
		esoInstall = v
	}
	xrr := isProd
	if v := dataCfg.Get("enableCrossRegionReplication"); v != "" {
		if v == "true" {
			xrr = true
		} else if v == "false" {
			xrr = false
		}
	}

	backupDays := 7
	kmsDeletion := 7
	postgresProvisioned := false
	postgresReaders := 0
	postgresInstance := ""
	postgresMin, postgresMax := 0.5, 8.0
	redisShards := 1
	redisReplicas := 1
	snapshotRetention := 5
	switch {
	case isProd:
		backupDays = 35
		kmsDeletion = 30
		postgresProvisioned = true
		postgresReaders = 2
		postgresInstance = "db.r6g.large"
		redisShards = 3
		redisReplicas = 2
		snapshotRetention = 5
	case stack == "staging":
		backupDays = 14
		redisShards = 1
		redisReplicas = 1
	}

	dbName := dataCfg.Get("dbName")
	if dbName == "" {
		dbName = "ironflyer"
	}
	dbUser := dataCfg.Get("dbUser")
	if dbUser == "" {
		dbUser = "ironflyer"
	}
	pgVersion := dataCfg.Get("postgresEngineVersion")
	if pgVersion == "" {
		pgVersion = "16.4"
	}
	redisVersion := dataCfg.Get("redisEngineVersion")
	if redisVersion == "" {
		redisVersion = "7.1"
	}
	backupWindow := dataCfg.Get("backupWindow")
	if backupWindow == "" {
		backupWindow = "02:00-03:00"
	}
	maintWindow := dataCfg.Get("maintenanceWindow")
	if maintWindow == "" {
		maintWindow = "sun:03:30-sun:04:30"
	}
	esoNS := dataCfg.Get("eso.namespace")
	if esoNS == "" {
		esoNS = "external-secrets"
	}
	appNS := dataCfg.Get("appNamespace")
	if appNS == "" {
		appNS = "ironflyer"
	}

	return &Env{
		Stack:                      stack,
		Region:                     region,
		ReplicaRegion:              dataCfg.Get("replicaRegion"),
		IsProd:                     isProd,
		IsDev:                      isDev,
		DBName:                     dbName,
		DBUser:                     dbUser,
		PostgresEngineVersion:      pgVersion,
		RedisEngineVersion:         redisVersion,
		BackupWindow:               backupWindow,
		MaintenanceWindow:          maintWindow,
		ESOInstall:                 esoInstall,
		ESONamespace:               esoNS,
		AppNamespace:               appNS,
		Kubeconfig:                 dataCfg.Get("kubeconfig"),
		CrossRegionReplication:     xrr,
		ComputeStackName:           computeStack,
		BackupRetentionDays:        backupDays,
		KMSDeletionWindowDays:      kmsDeletion,
		PostgresProvisioned:        postgresProvisioned,
		PostgresReaders:            postgresReaders,
		PostgresInstanceClass:      postgresInstance,
		PostgresMinACU:             postgresMin,
		PostgresMaxACU:             postgresMax,
		RedisShards:                redisShards,
		RedisReplicasPerShard:      redisReplicas,
		RedisSnapshotRetentionDays: snapshotRetention,
		Tags: pulumi.StringMap{
			"Project":   pulumi.String("ironflyer"),
			"Stack":     pulumi.String(stack),
			"ManagedBy": pulumi.String("pulumi"),
			"Layer":     pulumi.String("data"),
		},
	}, nil
}

// Name builds a deterministic Pulumi logical name "ironflyer-<stack>-<suffix>".
func (e *Env) Name(suffix string) string {
	return fmt.Sprintf("ironflyer-%s-%s", e.Stack, suffix)
}

// Alias builds a KMS alias "alias/ironflyer/<domain>/<stack>".
func (e *Env) Alias(domain string) string {
	return fmt.Sprintf("alias/ironflyer/%s/%s", domain, e.Stack)
}
