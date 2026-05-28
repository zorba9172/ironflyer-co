// Thin GraphQL fetcher with APQ support. Prod requires the persisted-query
// hash on every POST (see project memory: prod is APQ-locked). The hash map is
// injected at build by codegen; until then dev sends the full query.

export interface GraphQLClientOptions {
  endpoint: string;
  getToken?: () => string | null | undefined;
  persistedQueries?: Record<string, string>; // operationName -> sha256
}

export class GraphQLError extends Error {
  constructor(message: string, public readonly errors: unknown) {
    super(message);
    this.name = 'GraphQLError';
  }
}

export function createGraphQLClient(opts: GraphQLClientOptions) {
  return async function request<T>(
    operationName: string,
    query: string,
    variables?: Record<string, unknown>,
  ): Promise<T> {
    const token = opts.getToken?.();
    const sha = opts.persistedQueries?.[operationName];
    const body: Record<string, unknown> = { operationName, variables };
    if (sha) body.extensions = { persistedQuery: { version: 1, sha256Hash: sha } };
    else body.query = query;

    const res = await fetch(opts.endpoint, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        ...(token ? { authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify(body),
    });
    const json = (await res.json()) as { data?: T; errors?: unknown };
    if (json.errors) throw new GraphQLError(`GraphQL request ${operationName} failed`, json.errors);
    return json.data as T;
  };
}

export type GraphQLRequest = ReturnType<typeof createGraphQLClient>;
