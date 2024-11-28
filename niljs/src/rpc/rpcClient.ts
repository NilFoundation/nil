import { Client, HTTPTransport, RequestManager } from "@open-rpc/client-js";
import fetch from "isomorphic-fetch";
import { version } from "../version.js";

/**
 * The options for the RPC client.
 */
type RPCClientOptions = {
  signal?: AbortSignal;
};

/**
 * Creates a new RPC client to interact with the network using the RPC API.
 * The RPC client uses an HTTP transport to send requests to the network.
 * HTTP is currently the only supported transport.
 * @example const client = createRPCClient(RPC_ENDPOINT);
 */
const createRPCClient = (endpoint: string, { signal }: RPCClientOptions = {}) => {
  const fetcher: typeof fetch = (url, options) => {
    return fetch(url, { ...options, signal });
  };

  const transport = new HTTPTransport(endpoint, {
    headers: {
      "Client-Version": version,
    },
    fetcher,
  });

  const requestManager = new RequestManager([transport]);
  return new Client(requestManager);
};

export { createRPCClient };
