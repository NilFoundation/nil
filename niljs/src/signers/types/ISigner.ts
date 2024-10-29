/**
 * The interface for the Signer class
 */
interface ISigner {
  /**
   * Signs the data.
   * Accepts valid data as an argument and returns the signature.
   * @param data - The data to sign.
   * @returns The signature.
   * @example
   * const data = Uint8Array.from([0, 1, 2, 3, 4, 5, 6, 7, 8, 9]);
   * const recoveredSignature = signer.sign(data);
   */
  sign(data: Uint8Array): Promise<Uint8Array>;
  /**
   * Retrieves the public key.
   * @param shardId - The shard ID.
   * @returns The public key.
   * @example
   * const address = signer.getPublicKey();
   */
  getPublicKey(): Uint8Array;
  /**
   * Retrieves the address.
   * @param shardId - The shard ID.
   * @returns The address.
   * @example
   * const address = signer.getAddress();
   */
  getAddress(shardId: number): Uint8Array;
}

export type { ISigner };
