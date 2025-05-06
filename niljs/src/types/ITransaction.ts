/**
 * The interface for the transaction object. This object is used to represent a transaction in the client code.
 * It may differ from the actual transaction object used inside the network.
 */
export type ITransaction = {
  feeCredit: bigint;
  maxPriorityFeePerGas: bigint;
  maxFeePerGas: bigint;
  seqno: number;
  chainId: number;
  to: Uint8Array;
  data: Uint8Array;
  deploy: boolean;
};
