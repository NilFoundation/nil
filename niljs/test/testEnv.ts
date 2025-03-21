import "dotenv/config";

const defaultRpcEndpoint = "http://127.0.0.1:8529";
const defaultFaucetServiceEndpoint = "http://127.0.0.1:8529";
const defaultCometaClientEndpoint = "http://127.0.0.1:8529";

const testEnv = {
  endpoint: process.env.RPC_ENDPOINT ?? defaultRpcEndpoint,
  faucetServiceEndpoint: process.env.FAUCET_SERVICE_ENDPOINT ?? defaultFaucetServiceEndpoint,
  cometaClientEndpoint: process.env.COMETA_SERVICE_ENDPOINT ?? defaultCometaClientEndpoint,
} as const;

export { testEnv };
