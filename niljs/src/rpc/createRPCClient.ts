import { Client, HTTPTransport, RequestManager } from "@open-rpc/client-js";
import fetch from "isomorphic-fetch";
import { isValidHttpHeaders } from "../utils/rpc.js";
import { version } from "../version.js";

/**
 * The options for the RPC client.
 */
type RPCClientOptions = {
  headers?: Record<string, string>;
  fetcher?: typeof fetch;
};

/**
 * Creates a new RPC client to interact with the network using the RPC API.
 * The RPC client uses an HTTP transport to send requests to the network.
 * HTTP is currently the only supported transport.
 * @example const client = createRPCClient(RPC_ENDPOINT);
 */
const createRPCClient = (
  endpoint: string,
  { headers = {}, fetcher = fetch }: RPCClientOptions = {}
) => {
  isValidHttpHeaders(headers);

  const transport = new HTTPTransport(endpoint, {
    headers: {
      "Client-Version": `niljs/${version}`,
      ...headers,
    },
    fetcher,
  });

  const requestManager = new RequestManager([transport]);
  return new Client(requestManager);
};

export { createRPCClient };
