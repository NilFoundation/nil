import type { JSONRPCClient, JSONRPCRequest } from "json-rpc-2.0";
import { requestHeadersWithDefaults } from "../utils/rpc.js";
import type { IHttpTransportConfig } from "./types/IHttpTransportConfig.js";
import type { ITransport } from "./types/ITransport.js";

const getIsomorphicFetch = () => {
  if (typeof window !== "undefined") {
    return window.fetch.bind(window);
  }

  if (typeof globalThis !== "undefined") {
    return globalThis.fetch;
  }

  throw new Error("No fetch implementation found");
};

/**
 * HttpTransport represents the HTTP transport for connecting to the network.
 *
 * @class HttpTransport
 * @typedef {HttpTransport}
 * @implements {ITransport}
 */
class HttpTransport implements ITransport {
  /**
   * The endpoint to which the transport connects.
   */
  public endpoint: string;

  /**
   * The timeout for the requests.
   */
  public timeout: number;

  /**
   * The headers to be used in the requests.
   */
  public headers: Record<string, string>;

  /**
   * The fetcher to be used in the requests.
   */
  public fetcher: typeof fetch;

  constructor({ endpoint, timeout = 20000, headers, fetcher }: IHttpTransportConfig) {
    this.endpoint = endpoint;
    this.timeout = timeout;
    this.headers = requestHeadersWithDefaults(headers);
    this.fetcher = fetcher || getIsomorphicFetch();
  }

  /**
   * Sends a request to the network.
   *
   * @public
   * @async
   * @template T
   * @param {JSONRPCRequest} requestObject The request object.
   * @returns {Promise<T>} The response.
   */
  public async request(requestObject: JSONRPCRequest, client: JSONRPCClient) {
    const requestStringified = JSON.stringify(requestObject);

    const abortController = new AbortController();

    /**
     * The signal is used to abort the request if it takes too long.
     */
    const signal = AbortSignal.any([abortController.signal, AbortSignal.timeout(this.timeout)]);

    return this.fetcher(this.endpoint, {
      method: "POST",
      headers: this.headers,
      body: requestStringified,
      signal: signal,
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`);
        }

        return response.json();
      })
      .then((data) => {
        if (data.error) {
          throw new Error(`RPC error! message: ${data.error.message}`);
        }

        client.receive(data);
      })
      .catch((error) => {
        if (error.name === "AbortError") {
          throw new Error(`Request timed out after ${this.timeout}ms`);
        }

        throw error;
      })
      .finally(() => {
        abortController.abort();
      });
  }
}

export { HttpTransport };
