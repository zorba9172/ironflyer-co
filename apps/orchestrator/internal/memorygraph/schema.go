package memorygraph

// SurrealQL DDL for the V22 AI Memory Graph. Two tables — graph_nodes
// (vertices) and graph_edges (relations) — both SCHEMALESS so the
// projection code can evolve Attrs without a migration. Indexes cover
// the high-value queries: by tenant, by tenant+kind, and by canonical
// ref for idempotent upsert.
//
// Idempotent — every DEFINE statement uses IF NOT EXISTS so calling
// Bootstrap on every boot is safe.
const surrealGraphSchema = `
DEFINE TABLE IF NOT EXISTS graph_nodes SCHEMALESS;
DEFINE FIELD IF NOT EXISTS kind         ON TABLE graph_nodes TYPE string;
DEFINE FIELD IF NOT EXISTS tenantId     ON TABLE graph_nodes TYPE string;
DEFINE FIELD IF NOT EXISTS projectId    ON TABLE graph_nodes TYPE option<string>;
DEFINE FIELD IF NOT EXISTS canonicalRef ON TABLE graph_nodes TYPE option<string>;
DEFINE FIELD IF NOT EXISTS summary      ON TABLE graph_nodes TYPE option<string>;
DEFINE FIELD IF NOT EXISTS confidence   ON TABLE graph_nodes TYPE option<number>;
DEFINE FIELD IF NOT EXISTS attrs        ON TABLE graph_nodes TYPE option<object>;
DEFINE FIELD IF NOT EXISTS provenance   ON TABLE graph_nodes TYPE option<object>;
DEFINE FIELD IF NOT EXISTS createdAt    ON TABLE graph_nodes TYPE datetime;
DEFINE FIELD IF NOT EXISTS updatedAt    ON TABLE graph_nodes TYPE datetime;
DEFINE INDEX IF NOT EXISTS gnodes_byTenant         ON TABLE graph_nodes COLUMNS tenantId;
DEFINE INDEX IF NOT EXISTS gnodes_byTenantKind     ON TABLE graph_nodes COLUMNS tenantId, kind;
DEFINE INDEX IF NOT EXISTS gnodes_byTenantProject  ON TABLE graph_nodes COLUMNS tenantId, projectId;
DEFINE INDEX IF NOT EXISTS gnodes_byCanonicalRef   ON TABLE graph_nodes COLUMNS tenantId, kind, canonicalRef;
DEFINE INDEX IF NOT EXISTS gnodes_byUpdatedAt      ON TABLE graph_nodes COLUMNS updatedAt;

DEFINE TABLE IF NOT EXISTS graph_edges SCHEMALESS;
DEFINE FIELD IF NOT EXISTS kind        ON TABLE graph_edges TYPE string;
DEFINE FIELD IF NOT EXISTS tenantId    ON TABLE graph_edges TYPE string;
DEFINE FIELD IF NOT EXISTS fromId      ON TABLE graph_edges TYPE string;
DEFINE FIELD IF NOT EXISTS toId        ON TABLE graph_edges TYPE string;
DEFINE FIELD IF NOT EXISTS weight      ON TABLE graph_edges TYPE option<number>;
DEFINE FIELD IF NOT EXISTS confidence  ON TABLE graph_edges TYPE option<number>;
DEFINE FIELD IF NOT EXISTS inferred    ON TABLE graph_edges TYPE option<bool>;
DEFINE FIELD IF NOT EXISTS attrs       ON TABLE graph_edges TYPE option<object>;
DEFINE FIELD IF NOT EXISTS provenance  ON TABLE graph_edges TYPE option<object>;
DEFINE FIELD IF NOT EXISTS createdAt   ON TABLE graph_edges TYPE datetime;
DEFINE INDEX IF NOT EXISTS gedges_byTenant       ON TABLE graph_edges COLUMNS tenantId;
DEFINE INDEX IF NOT EXISTS gedges_byFrom         ON TABLE graph_edges COLUMNS tenantId, fromId, kind;
DEFINE INDEX IF NOT EXISTS gedges_byTo           ON TABLE graph_edges COLUMNS tenantId, toId, kind;
DEFINE INDEX IF NOT EXISTS gedges_byKind         ON TABLE graph_edges COLUMNS tenantId, kind;
`

// SurrealGraphSchema is exported so the integration agent can choose to
// bootstrap the schema lazily (e.g. only when IRONFLYER_DB_DRIVER selects
// the surreal backend). Most callers should just call Graph.Bootstrap.
func SurrealGraphSchema() string { return surrealGraphSchema }
