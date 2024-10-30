const defaultRpcEndpoint = "http://127.0.0.1:8529";
const defaultFaucetServiceEndpoint = "http://127.0.0.1:8527";
const defaultCometaServiceEndpoint = "http://127.0.0.1:8528";

const testEnv = {
  endpoint: process.env.RPC_ENDPOINT ?? defaultRpcEndpoint,
  faucetServiceEndpoint: process.env.FAUCET_SERVICE_ENDPOINT ?? defaultFaucetServiceEndpoint,
  cometaServiceEndpoint: process.env.COMETA_SERVICE_ENDPOINT ?? defaultCometaServiceEndpoint,
} as const;

export { testEnv };
