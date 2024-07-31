import { Client, HTTPTransport, RequestManager } from "@open-rpc/client-js";

/**
 * Creates a new RPC client to interact with the network using the RPC API.
 * The RPC client uses an HTTP transport to send requests to the network.
 * HTTP is currently the only supported transport.
 * @example const client = createRPCClient("http://127.0.0.1:8529");
 */
const createRPCClient = (endpoint: string) => {
  const transport = new HTTPTransport(endpoint);
  const requestManager = new RequestManager([transport]);
  return new Client(requestManager);
};

export { createRPCClient };
