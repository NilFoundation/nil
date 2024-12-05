import { Faucet as FaucetSol} from "@nilfoundation/smart-contracts";
import { type Hex, bytesToHex, encodeFunctionData } from "viem";
import type { PublicClient } from "../../clients/PublicClient.js";
import { ExternalMessageEnvelope } from "../../encoding/externalMessage.js";
import { hexToBytes } from "../../index.js";
import { waitTillCompleted } from "../../utils/receipt.js";

/**
 * The Faucet is a special contract that is used to top up other contracts in the =nil; devnet.
 *
 * @class Faucet
 * @typedef {Faucet}
 */
export class Faucet {
  /**
   * The const address of the Faucet contract.
   *
   * @static
   * @type {"0x000100000000000000000000000000000FA00CE7"}
   */
  static address = "0x000100000000000000000000000000000FA00CE7" as const;
  /**
   * The client to be used with the Faucet contract.
   *
   * @private
   * @type {PublicClient}
   */
  private client: PublicClient;

  /**
   * Creates an instance of Faucet.
   *
   * @constructor
   * @param {PublicClient} client
   */
  constructor(client: PublicClient) {
    this.client = client;
  }

  /**
   * Withdraws the specified value to the given address.
   *
   * @deprecated
   * @async
   * @param {Hex} address The address to which the withdrawal should be made.
   * @param {bigint} [value=1000000000000000000n] The value that should be withdrawn to the given address.
   * @param {?number} [seqno] The sequence number of the withdrawal message.
   * @returns {Uint8Array} The hash of the withdrawal message.
   */
  async withdrawTo(address: Hex, value = 1000000000000000000n, seqno?: number) {
    const [refinedSeqno, chainId] = await Promise.all([
      seqno ?? this.client.getMessageCount(Faucet.address, "latest"),
      this.client.chainId(),
    ]);
    const calldata = encodeFunctionData({
      abi: FaucetSol.abi,
      functionName: "withdrawTo",
      args: [address.toLowerCase() as Hex, value],
    });
    const message = new ExternalMessageEnvelope({
      isDeploy: false,
      to: hexToBytes(Faucet.address),
      chainId,
      seqno: refinedSeqno,
      data: hexToBytes(calldata),
      authData: new Uint8Array(0),
    });
    const encodedMessage = message.encode();
    await this.client.sendRawMessage(bytesToHex(encodedMessage));
    return message.hash();
  }

  /**
   * Withdraws the specified value to the given address with retries.
   *
   * @async
   * @param {Hex} address The address to which the withdrawal should be made.
   * @param {bigint} [value=1000000000000000000n] The value that should be withdrawn to the given address.
   * @param {?number} [retries=3] How many times to retry the withdrawal in case of failure.
   * @returns {Uint8Array} The hash of the withdrawal message.
   */
  async withdrawToWithRetry(address: Hex, value = 1000000000000000000n, retries = 5) {
    let currentRetries = 0;
    while (currentRetries++ < retries) {
      try {
        const [refinedSeqno, chainId] = await Promise.all([
          this.client.getMessageCount(Faucet.address, "latest"),
          this.client.chainId(),
        ]);
        const calldata = encodeFunctionData({
          abi: FaucetSol.abi,
          functionName: "withdrawTo",
          args: [address.toLowerCase() as Hex, value],
        });
        const message = new ExternalMessageEnvelope({
          isDeploy: false,
          to: hexToBytes(Faucet.address),
          chainId,
          seqno: refinedSeqno,
          data: hexToBytes(calldata),
          authData: new Uint8Array(0),
        });
        const encodedMessage = message.encode();
        await this.client.sendRawMessage(bytesToHex(encodedMessage));
        const hash = bytesToHex(message.hash());
        const receipts = await Promise.race([
          new Promise<[]>((resolve) => setTimeout(() => resolve([]), 10000)),
          waitTillCompleted(this.client, hash),
        ]);
        if (receipts.length === 0) {
          continue;
        }
        if (receipts.some((receipt) => !receipt.success)) {
          continue;
        }
        return hash;
      } catch (error) {
        await new Promise((resolve) => setTimeout(resolve, 1000));
        if (currentRetries >= retries) {
          throw error;
        }
      }
    }
    throw new Error("Failed to withdraw to the given address");
  }
}
