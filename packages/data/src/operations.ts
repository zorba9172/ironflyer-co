// Real orchestrator operations (subset), matching packages/sdk/src/operations.graphql
// and the GraphQL schema. Used by the data hooks when an endpoint is configured.

export const ME = /* GraphQL */ `query Me { me { id email name plan } }`;

export const SIGN_IN = /* GraphQL */ `
  mutation SignIn($input: SignInInput!) {
    signIn(input: $input) { token expiresAt user { id email name plan } }
  }`;

export const SIGN_UP = /* GraphQL */ `
  mutation SignUp($input: SignUpInput!) {
    signUp(input: $input) { token expiresAt user { id email name plan } }
  }`;

export const SIGN_OUT = /* GraphQL */ `mutation SignOut { signOut { ok } }`;

export const PROJECTS = /* GraphQL */ `
  query Projects { projects { id name status idea updatedAt } }`;

export const PROJECT_SNAPSHOT = /* GraphQL */ `
  query ProjectSnapshot($id: ID!) { projectSnapshot(id: $id) }`;

export const RUN_FINISHER = /* GraphQL */ `mutation RunFinisher($id: ID!) { runFinisher(id: $id) }`;

export const APPLY_PATCH = /* GraphQL */ `
  mutation ApplyPatch($patchId: ID!) { applyPatch(patchId: $patchId) { id title state lines } }`;
