const defaultRpcEndpoint = "http://127.0.0.1:8529";
const defaultFaucetServiceEndpoint = "http://127.0.0.1:8529";
const defaultCometaServiceEndpoint = "http://127.0.0.1:8529";
const defaultNildBinary = "nild";
const testEnv = {
  endpoint: process.env.RPC_ENDPOINT ?? defaultRpcEndpoint,
  faucetServiceEndpoint: process.env.FAUCET_SERVICE_ENDPOINT ?? defaultFaucetServiceEndpoint,
  cometaServiceEndpoint: process.env.COMETA_SERVICE_ENDPOINT ?? defaultCometaServiceEndpoint,
  nild: process.env.NILD ?? defaultNildBinary,
} as const;

export { testEnv };
