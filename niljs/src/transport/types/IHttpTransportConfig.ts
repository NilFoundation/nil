/**
 * The interface representing the configuration of the HTTP transport.
 */
type IHttpTransportConfig = {
  /**
   * The network endpoint. It is set to the URL of the network node.
   * @example 'http://127.0.0.1:8529'
   */
  endpoint: string;
  /**
   * The request timeout.
   * If the request is not completed within the timeout, it will be rejected.
   * @example 1000
   * @default 20000
   */
  timeout?: number;
  /**
   * The fetch function to be used for making requests.
   * This is useful for testing purposes and leveraging signals/etc.
   * @example
   * import fetch from 'isomorphic-fetch';
   * @default fetch
   */
  fetcher?: typeof fetch;
  /**
   * The headers to be sent with the request.
   * @example { 'My-header': 'my-value' }
   * @default {}
   */
  headers?: Record<string, string>;
};

export type { IHttpTransportConfig };
