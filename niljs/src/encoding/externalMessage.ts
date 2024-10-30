import { numberToBytesBE } from "@noble/curves/abstract/utils";
import type { PublicClient } from "../clients/PublicClient.js";
import type { ISigner } from "../signers/index.js";
import type { ExternalMessage } from "../types/ExternalMessage.js";
import type { IDeployData } from "../types/IDeployData.js";
import { prepareDeployPart } from "./deployPart.js";
import { bytesToHex } from "./fromBytes.js";
import { poseidonHash } from "./poseidon.js";
import { SszMessageSchema, SszSignedMessageSchema } from "./ssz.js";

/**
 * The envelope for an external message (a message sent by a user, a dApp, etc.)
 *
 * @class ExternalMessageEnvelope
 * @typedef {ExternalMessageEnvelope}
 */
export class ExternalMessageEnvelope {
  /**
   * The flag that determines whether the external message is a deployment message.
   *
   * @type {boolean}
   */
  isDeploy: boolean;
  /**
   * The destination address of the message.
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
   * The message sequence number.
   *
   * @type {number}
   */
  seqno: number;
  /**
   * The message data.
   *
   * @type {Uint8Array}
   */
  data: Uint8Array;
  /**
   * The auth data attached to the message.
   *
   * @type {Uint8Array}
   */
  authData: Uint8Array;
  /**
   * Creates an instance of ExternalMessageEnvelope.
   *
   * @constructor
   * @param {ExternalMessage} param0 The object representing the external message.
   * @param {ExternalMessage} param0.isDeploy The flag that determines whether the external message is a deployment message.
   * @param {ExternalMessage} param0.to The destination address of the message.
   * @param {ExternalMessage} param0.chainId The chain ID.
   * @param {ExternalMessage} param0.seqno The message sequence number.
   * @param {ExternalMessage} param0.data The message number.
   * @param {ExternalMessage} param0.authData The auth data attached to the message.
   */
  constructor({ isDeploy, to, chainId, seqno, data, authData }: ExternalMessage) {
    this.isDeploy = isDeploy;
    this.to = to;
    this.chainId = chainId;
    this.seqno = seqno;
    this.data = data;
    this.authData = authData;
  }
  /**
   * Encodes the external message into a Uint8Array.
   *
   * @public
   * @returns {Uint8Array} The encoded external message.
   */
  public encode(): Uint8Array {
    return SszSignedMessageSchema.serialize({
      feeCredit: 50000000n,
      seqno: this.seqno,
      chainId: this.chainId,
      to: this.to,
      data: this.data,
      deploy: this.isDeploy,
      authData: this.authData,
    });
  }
  /**
   * Provides the hash tree root of the external message.
   *
   * @public
   * @returns {Uint8Array} The hash tree root of the external message.
   */
  public hash(): Uint8Array {
    const raw = this.encode();
    return numberToBytesBE(poseidonHash(raw), 32);
  }
  /**
   * Provides the signing hash of the external message.
   *
   * @public
   * @returns {Uint8Array} The signing hash of the external message.
   */
  public signingHash(): Uint8Array {
    // print all the fields
    const raw = SszMessageSchema.serialize({
      feeCredit: 50000000n,
      seqno: this.seqno,
      chainId: this.chainId,
      to: this.to,
      data: this.data,
      deploy: this.isDeploy,
    });
    return numberToBytesBE(poseidonHash(raw), 32);
  }
  /**
   * Encodes the external message with its signature.
   *
   * @public
   * @async
   * @param {ISigner} signer The message signer.
   * @returns {Promise<{
   *     raw: Uint8Array;
   *     hash: Uint8Array;
   *   }>} The object containing the encoded message and its hash.
   */
  public async encodeWithSignature(signer: ISigner): Promise<{
    raw: Uint8Array;
    hash: Uint8Array;
  }> {
    const signature = await this.sign(signer);
    const raw = SszSignedMessageSchema.serialize({
      feeCredit: 50000000n,
      seqno: this.seqno,
      chainId: this.chainId,
      to: this.to,
      data: this.data,
      deploy: this.isDeploy,
      authData: signature,
    });
    const hash = numberToBytesBE(poseidonHash(raw), 32);
    return { raw, hash };
  }
  /**
   * Signs the external message.
   *
   * @public
   * @async
   * @param {ISigner} signer The message signer.
   * @returns {Promise<Uint8Array>} The message signature.
   */
  public async sign(signer: ISigner): Promise<Uint8Array> {
    return signer.sign(this.signingHash());
  }
  /**
   * Updates the authentication data in the external message and returns the result.
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
   * Sends the external message.
   *
   * @public
   * @param {PublicClient} client The client sending the message.
   * @returns {*} The hash of the external message.
   */
  public send(client: PublicClient) {
    return client.sendRawMessage(this.encode());
  }
}

/**
 * The envelope for an internal message (a message sent by a smart contract to another smart contract).
 *
 * @class InternalMessageEnvelope
 * @typedef {InternalMessageEnvelope}
 */
export class InternalMessageEnvelope {}

/**
 * Creates a new external deployment message.
 *
 * @param {IDeployData} data The message data.
 * @param {number} chainId The chain ID.
 * @returns {ExternalMessageEnvelope} The envelope of the external deployment message.
 * @example
 * import {
     Faucet,
     LocalECDSAKeySigner,
     HttpTransport,
     PublicClient
     WalletV1,
     externalDeploymentMessage,
     generateRandomPrivateKey,
   } from '@nilfoundation/niljs';
 * const signer = new LocalECDSAKeySigner({
     privateKey: generateRandomPrivateKey(),
   });

   const pubkey = signer.getPublicKey();
 * const chainId = await client.chainId();
 * const deploymentMessage = externalDeploymentMessage(
     {
       salt: 100n,
       shard: 1,
       bytecode: WalletV1.code,
       abi: WalletV1.abi,
       args: [bytesToHex(pubkey)],
     },
     chainId,
   );
 */
export const externalDeploymentMessage = (
  data: IDeployData,
  chainId: number,
): ExternalMessageEnvelope => {
  const { data: deployData, address } = prepareDeployPart(data);
  return new ExternalMessageEnvelope({
    isDeploy: true,
    to: address,
    chainId,
    seqno: 0,
    data: deployData,
    authData: new Uint8Array(0),
  });
};

/**
 * Encodes the given external message.
 *
 * @async
 * @param {Omit<ExternalMessage, "authData">} params The external message to be encoded without its auth data.
 * @param {ISigner} signer The message signer.
 * @returns {Promise<{ raw: Uint8Array; hash: Uint8Array }>} The message bytecode and the message hash.
 */
export const externalMessageEncode = async (
  params: Omit<ExternalMessage, "authData">,
  signer: ISigner,
): Promise<{ raw: Uint8Array; hash: Uint8Array }> => {
  const message = new ExternalMessageEnvelope({
    ...params,
    authData: new Uint8Array(0),
  });
  const res = await message.encodeWithSignature(signer);
  return res;
};
