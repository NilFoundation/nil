const defaultRpcEndpoint = "http://127.0.0.1:8529";

const testEnv = {
  endpoint: process.env.RPC_ENDPOINT ?? defaultRpcEndpoint,
} as const;

export { testEnv };
