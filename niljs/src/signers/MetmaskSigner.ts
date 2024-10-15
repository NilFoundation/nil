import invariant from "tiny-invariant";
import type { ISigner } from "./types/ISigner.js";

/**
 * The MetaMaskSigner is a class that allows for signing data using MetaMask.
 * This signer can only be used inside a browser.
 * @example
 * import { MetaMaskSigner } from '@nilfoundation/niljs';
 *
 * const signer = new MetaMaskSigner();
 */
class MetaMaskSigner implements ISigner {
  /**
   * The signer provider.
   *
   * @private
   * @type {*}
   */
  private provider;
  /**
   * Creates an instance of MetaMaskSigner.
   *
   * @constructor
   */
  constructor() {
    invariant(
      typeof window !== "undefined",
      "MetaMaskSigner can be used in the browser only",
    );

    invariant(
      typeof window.ethereum !== "undefined",
      "No MetaMask provider found. Please install MetaMask browser extension before using MetaMaskSigner",
    );

    this.provider = window.ethereum;
  }

  /**
   * Signs the data.
   *
   * @public
   * @async
   * @param {Uint8Array} data The input data.
   * @returns {unknown} The signed data.
   */
  public async sign(_: Uint8Array) {
    // mock signature
    return new Uint8Array(65);
  }

  /**
   * Retrieves the wallet address.
   *
   * @public
   * @async
   * @returns {unknown}
   */
  public async getAddress() {
    // mock address
    return new Uint8Array(20);
  }

  /**
   * Retreieves the public key.
   *
   * @public
   * @async
   * @returns {unknown}
   */
  public async getPublicKey() {
    // mock public key
    return new Uint8Array(33);
  }

  /**
   * Connects the signer to an existing MetaMask wallet.
   *
   * @public
   * @async
   * @returns {Promise<void>}
   */
  public async connect(): Promise<void> {
    await this.provider.request({ method: "eth_requestAccounts" });
  }
}

export { MetaMaskSigner };
