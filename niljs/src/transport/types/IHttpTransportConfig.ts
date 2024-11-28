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
   * The signal to abort the request.
   * If the signal is aborted, the request will be aborted.
   * @example new AbortSignal()
   * @default undefined
   */
  signal?: AbortSignal;
};

export type { IHttpTransportConfig };
