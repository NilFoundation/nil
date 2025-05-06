import { encode as rlpEncode } from "@ethereumjs/rlp";
import { numberToBytesBE } from "@noble/curves/abstract/utils";
import { keccak_256 } from "@noble/hashes/sha3";
import type { PublicClient } from "../clients/PublicClient.js";
import type { ISigner } from "../signers/types/ISigner.js";
import type { ExternalTransaction } from "../types/ExternalTransaction.js";
import type { IDeployData } from "../types/IDeployData.js";
import { getShardIdFromAddress } from "../utils/address.js";
import { prepareDeployPart } from "./deployPart.js";
import { bytesToHex } from "./fromBytes.js";

function serializeTransactionRLP(tx: ExternalTransactionEnvelope): Uint8Array {
  return rlpEncode([
    tx.isDeploy ? 1 : 0,
    [tx.feeCredit, tx.maxPriorityFeePerGas, tx.maxFeePerGas],
    tx.to,
    tx.chainId,
    tx.seqno,
    tx.data,
  ]);
}

function serializeSignedTransactionRLP(tx: ExternalTransactionEnvelope): Uint8Array {
  return rlpEncode([
    tx.isDeploy ? 1 : 0,
    [tx.feeCredit, tx.maxPriorityFeePerGas, tx.maxFeePerGas],
    tx.to,
    tx.chainId,
    tx.seqno,
    tx.data,
    tx.authData,
  ]);
}

/**
 * The envelope for an external transaction (a transaction sent by a user, a dApp, etc.)
 *
 * @class ExternalTransactionEnvelope
 * @typedef {ExternalTransactionEnvelope}
 */
export class ExternalTransactionEnvelope {
  /**
   * The flag that determines whether the external transaction is a deployment transaction.
   *
   * @type {boolean}
   */
  isDeploy: boolean;
  /**
   * The destination address of the transaction.
   *
   * @type {Uint8Array}
   */
  to: Uint8Array;
  /**
   * The chain ID.
   *
   * @type {number}
   */
  chainId: number;
  /**
   * The transaction sequence number.
   *
   * @type {number}
   */
  seqno: number;
  /**
   * The transaction data.
   *
   * @type {Uint8Array}
   */
  data: Uint8Array;
  /**
   * The auth data attached to the transaction.
   *
   * @type {Uint8Array}
   */
  authData: Uint8Array;
  /**
   * The amount of tokens the user is willing to pay for the transaction.
   *
   * @type {BigInt}
   */
  feeCredit: bigint;
  /**
   * The max tip the user is willing to pay for the transaction.
   *
   * @type {BigInt}
   */
  maxPriorityFeePerGas: bigint;
  /**
   * The max fee per gas the user is willing to spend for the transaction.
   *
   * @type {BigInt}
   */
  maxFeePerGas: bigint;
  /**
   * Creates an instance of ExternalTransactionEnvelope.
   *
   * @constructor
   * @param {ExternalTransaction} param0 The object representing the external transaction.
   * @param {ExternalTransaction} param0.isDeploy The flag that determines whether the external transaction is a deployment transaction.
   * @param {ExternalTransaction} param0.to The destination address of the transaction.
   * @param {ExternalTransaction} param0.chainId The chain ID.
   * @param {ExternalTransaction} param0.seqno The transaction sequence number.
   * @param {ExternalTransaction} param0.data The transaction number.
   * @param {ExternalTransaction} param0.authData The auth data attached to the transaction.
   * @param {ExternalTransaction} param0.feeCredit The fee credit attached to the transaction.
   */
  constructor({
    isDeploy,
    to,
    chainId,
    seqno,
    data,
    authData,
    // TODO: feeCredit should not be a constant, it should be calculated based on the gas price
    feeCredit = 5_000_000n * 1_000_000n,
    maxPriorityFeePerGas = 0n,
    maxFeePerGas = feeCredit,
  }: ExternalTransaction) {
    this.isDeploy = isDeploy;
    this.to = to;
    this.chainId = chainId;
    this.seqno = seqno;
    this.data = data;
    this.authData = authData;
    this.feeCredit = feeCredit;
    this.maxPriorityFeePerGas = maxPriorityFeePerGas;
    this.maxFeePerGas = maxFeePerGas;
  }
  /**
   * Encodes the external transaction into a Uint8Array.
   *
   * @public
   * @returns {Uint8Array} The encoded external transaction.
   */
  public encode(): Uint8Array {
    return serializeSignedTransactionRLP(this);
  }
  /**
   * Provides the hash tree root of the external transaction.
   *
   * @public
   * @returns {Uint8Array} The hash tree root of the external transaction.
   */
  public hash(): Uint8Array {
    const raw = this.encode();
    const shardIdPart = numberToBytesBE(getShardIdFromAddress(bytesToHex(this.to)), 2);
    const hashPart = keccak_256(raw);
    return new Uint8Array([...shardIdPart, ...hashPart.slice(2)]);
  }
  /**
   * Provides the signing hash of the external transaction.
   *
   * @public
   * @returns {Uint8Array} The signing hash of the external transaction.
   */
  public signingHash(): Uint8Array {
    // print all the fields
    const raw = serializeTransactionRLP(this);
    return keccak_256(raw);
  }
  /**
   * Encodes the external transaction with its signature.
   *
   * @public
   * @async
   * @param {ISigner} signer The transaction signer.
   * @returns {Promise<{
   *     raw: Uint8Array;
   *     hash: Uint8Array;
   *   }>} The object containing the encoded transaction and its hash.
   */
  public async encodeWithSignature(signer: ISigner): Promise<{
    raw: Uint8Array;
    hash: Uint8Array;
  }> {
    const signature = await this.sign(signer);
    this.authData = signature;
    const raw = serializeSignedTransactionRLP(this);
    const shardIdPart = numberToBytesBE(getShardIdFromAddress(bytesToHex(this.to)), 2);
    const hashPart = keccak_256(raw);
    const hash = new Uint8Array([...shardIdPart, ...hashPart.slice(2)]);
    return { raw, hash };
  }
  /**
   * Signs the external transaction.
   *
   * @public
   * @async
   * @param {ISigner} signer The transaction signer.
   * @returns {Promise<Uint8Array>} The transaction signature.
   */
  public async sign(signer: ISigner): Promise<Uint8Array> {
    return signer.sign(this.signingHash());
  }
  /**
   * Updates the authentication data in the external transaction and returns the result.
   *
   * @public
   * @async
   * @param {ISigner} signer The auth data signer.
   * @returns {Promise<Uint8Array>} The signed auth data.
   */
  public async updateAuthdata(signer: ISigner): Promise<Uint8Array> {
    this.authData = await this.sign(signer);
    return this.authData;
  }
  /**
   * Returns the hex address of the given bytes.
   *
   * @public
   * @returns {`0x${string}`} The hex address.
   */
  public hexAddress(): `0x${string}` {
    return bytesToHex(this.to);
  }
  /**
   * Sends the external transaction.
   *
   * @public
   * @param {PublicClient} client The client sending the transaction.
   * @returns {*} The hash of the external transaction.
   */
  public send(client: PublicClient) {
    return client.sendRawTransaction(this.encode());
  }
}

/**
 * The envelope for an internal transaction (a transaction sent by a smart contract to another smart contract).
 *
 * @class InternalTransactionEnvelope
 * @typedef {InternalTransactionEnvelope}
 */
export class InternalTransactionEnvelope {}

/**
 * Creates a new external deployment transaction.
 *
 * @param {IDeployData} data The transaction data.
 * @param {number} chainId The chain ID.
 * @returns {ExternalTransactionEnvelope} The envelope of the external deployment transaction.
 * @example
 * import {
     Faucet,
     LocalECDSAKeySigner,
     HttpTransport,
     PublicClient
     SmartAccountV1,
     externalDeploymentTransaction,
     generateRandomPrivateKey,
   } from '@nilfoundation/niljs';
 * const signer = new LocalECDSAKeySigner({
     privateKey: generateRandomPrivateKey(),
   });

   const pubkey = signer.getPublicKey();
 * const chainId = await client.chainId();
 * const deploymentTransaction = externalDeploymentTransaction(
     {
       salt: 100n,
       shard: 1,
       bytecode: SmartAccountV1.code,
       abi: SmartAccountV1.abi,
       args: [bytesToHex(pubkey)],
     },
     chainId,
   );
 */
export const externalDeploymentTransaction = (
  data: IDeployData,
  maybeChainId?: number,
): ExternalTransactionEnvelope => {
  const { data: deployData, address } = prepareDeployPart(data);
  const chainId = data.chainId ?? maybeChainId;
  if (chainId === undefined) {
    throw new Error("Chain ID is required");
  }

  return new ExternalTransactionEnvelope({
    isDeploy: true,
    to: address,
    chainId,
    seqno: 0,
    data: deployData,
    authData: new Uint8Array(0),
    feeCredit: data.feeCredit ?? 5_000_000_000_000n,
  });
};

/**
 * Encodes the given external transaction.
 *
 * @async
 * @param {Omit<ExternalTransaction, "authData">} params The external transaction to be encoded without its auth data.
 * @param {ISigner} signer The transaction signer.
 * @returns {Promise<{ raw: Uint8Array; hash: Uint8Array }>} The transaction bytecode and the transaction hash.
 */
export const externalTransactionEncode = async (
  params: Omit<ExternalTransaction, "authData">,
  signer: ISigner,
): Promise<{ raw: Uint8Array; hash: Uint8Array }> => {
  const transaction = new ExternalTransactionEnvelope({
    ...params,
    authData: new Uint8Array(0),
  });
  const res = await transaction.encodeWithSignature(signer);
  return res;
};
