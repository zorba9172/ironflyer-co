import type { CodegenConfig } from "@graphql-codegen/cli";

// GraphQL Codegen — reads the orchestrator schema (source of truth)
// and the per-domain operation documents under src/lib/gql/operations,
// emits a single typed module that the cockpit + downstream pages
// (A47–A50) import for queries, mutations, subscriptions, and React
// hooks. Re-run with `npm run codegen` whenever a schema file or an
// operation document changes.
const config: CodegenConfig = {
  schema: "../orchestrator/internal/graph/schema/*.graphql",
  documents: "src/lib/gql/operations/**/*.graphql",
  ignoreNoDocuments: true,
  generates: {
    "src/lib/gql/__generated__.ts": {
      plugins: [
        "typescript",
        "typescript-operations",
        "typescript-react-apollo",
      ],
      config: {
        withHooks: true,
        reactApolloVersion: 3,
        skipTypename: false,
        avoidOptionals: { field: true },
        scalars: {
          JSON: "unknown",
          DateTime: "string",
          Decimal: "string",
          Bytes: "string",
        },
      },
    },
  },
};

export default config;
