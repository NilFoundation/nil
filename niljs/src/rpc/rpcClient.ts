import { Client, HTTPTransport, RequestManager } from "@open-rpc/client-js";
import { version } from "../version.js";

/**
 * Creates a new RPC client to interact with the network using the RPC API.
 * The RPC client uses an HTTP transport to send requests to the network.
 * HTTP is currently the only supported transport.
 * @example const client = createRPCClient(RPC_ENDPOINT);
 */
const createRPCClient = (endpoint: string) => {
	const transport = new HTTPTransport(endpoint, {
		headers: {
			"Client-Version": version,
		},
	});

	const requestManager = new RequestManager([transport]);
	return new Client(requestManager);
};

export { createRPCClient };
