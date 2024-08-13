import type { RequestArguments } from "@open-rpc/client-js/build/ClientInterface.js";
import invariant from "tiny-invariant";
import type { IHttpTransportConfig } from "./types/IHttpTransportConfig.js";
import type { ITransport } from "./types/ITransport.js";

/**
 * MetaMask transport represents the MetaMask transport for connecting to the network.
 * MetaMask transport can be used in browser only.
 * @class MetaMaskTransport
 * @typedef {MetaMaskTransport}
 * @implements {ITransport}
 */
class MetaMaskTransport implements ITransport {
  /**
   * The provider for the transport.
   *
   * @private
   * @type {*}
   */
  private provider;
  /**
   * The request timeout.
   *
   * @private
   * @type {number}
   */
  private timeout: number;

  /**
   * Creates an instance of MetaMaskTransport.
   *
   * @constructor
   * @param {IHttpTransportConfig} config The transport config.
   */
  constructor(config: IHttpTransportConfig) {
    this.timeout = config.timeout !== undefined ? config.timeout : 20000;

    invariant(
      typeof window !== "undefined",
      "MetaMaskTransport can be used in the browser only",
    );

    invariant(
      typeof window.ethereum !== "undefined",
      "No MetaMask provider found. Please install MetaMask browser extension before using MetaMaskSigner",
    );

    this.provider = window.ethereum;
  }

  /**
   * Sends a request to the network.
   *
   * @public
   * @async
   * @template T
   * @param {RequestArguments} requestObject The request object.
   * @returns {Promise<T>} The response.
   */
  public async request<T>(requestObject: RequestArguments): Promise<T> {
    return this.provider.request(requestObject);
  }

  /**
   * Connects to the network.
   *
   * @public
   */
  public connect(): void {
    this.provider.request({ method: "eth_requestAccounts" });
  }

  /**
   * Closes the connection to the network.
   *
   * @public
   */
  public closeConnection(): void {
    //
  }
}

export { MetaMaskTransport };
