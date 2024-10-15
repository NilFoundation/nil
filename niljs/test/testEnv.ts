const defaultRpcEndpoint = "http://127.0.0.1:8529";
const defaultFaucetServiceEndpoint = "http://127.0.0.1:8527";

const testEnv = {
  endpoint: process.env.RPC_ENDPOINT ?? defaultRpcEndpoint,
  faucetServiceEndpoint: process.env.FAUCET_SERVICE_ENDPOINT ?? defaultFaucetServiceEndpoint,
} as const;

export { testEnv };
