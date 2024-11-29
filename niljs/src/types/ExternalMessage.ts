/**
 * The external message type.
 *
 * @typedef {ExternalMessage}
 */
type ExternalMessage = {
  isDeploy: boolean;
  to: Uint8Array;
  chainId: number;
  seqno: number;
  data: Uint8Array;
  authData: Uint8Array;
  feeCredit?: bigint;
};

export type { ExternalMessage };
