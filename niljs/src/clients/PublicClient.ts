import {
  bytesToHex,
  hexToBigInt,
  hexToBytes,
  hexToNumber,
  toHex,
} from "../encoding/index.js";
import { BlockNotFoundError } from "../errors/block.js";
import { type Hex, assertIsValidShardId } from "../index.js";
import type { IAddress } from "../signers/types/IAddress.js";
import type { Block, BlockTag } from "../types/Block.js";
import type { CallArgs, CallRes, ContractOverride } from "../types/CallArgs.js";
import type { IReceipt, ProcessedReceipt } from "../types/IReceipt.js";
import type { ProcessedMessage } from "../types/ProcessedMessage.js";
import type { RPCMessage } from "../types/RPCMessage.js";
import { addHexPrefix } from "../utils/hex.js";
import { BaseClient } from "./BaseClient.js";
import type { IPublicClientConfig } from "./types/ClientConfigs.js";
import type { Address } from "abitype";
import { decodeFunctionResult, encodeFunctionData } from "viem";

/**
 * PublicClient is a class that allows for interacting with the network via the JSON-RPC API.
 * It provides an abstraction of the connection to =nil;.
 * PublicClient enables using API requests that do not require signing data (or otherwise using one's private key).
 * @example
 * import { PublicClient } from '@nilfoundation/niljs';
 *
 * const client = new PublicClient({
 *   transport: new HttpTransport({
 *     endpoint: RPC_ENDPOINT,
 *   }),
 *   shardId: 1,
 * });
 */
class PublicClient extends BaseClient {
  /**
   * Creates an instance of PublicClient.
   *
   * @constructor
   * @param {IPublicClientConfig} config The config to be used in the client. See {@link IPublicClientConfig}.
   */
  // biome-ignore lint/complexity/noUselessConstructor: may be useful in the future
  constructor(config: IPublicClientConfig) {
    super(config);
  }

  /**
   * Returns the block with the given hash.
   * @param hash The hash of the block whose information is requested.
   * @param fullTx The flag that determines whether full transaction information is returned in the output.
   * @param shardId The ID of the shard where the block was generated.
   * @returns Information about the block with the given hash.
   * @example
   * import { PublicClient } from '@nilfoundation/niljs';
   *
   * const client = new PublicClient({
   *   transport: new HttpTransport({
   *     endpoint: RPC_ENDPOINT,
   *   }),
   *   shardId: 1,
   * });
   *
   * const block = await client.getBlockByHash(HASH);
   */
  public async getBlockByHash(
    hash: Hex,
    fullTx = false,
    shardId = this.shardId,
  ) {
    assertIsValidShardId(shardId);

    try {
      return await this.request<Block<typeof fullTx>>({
        method: "eth_getBlockByHash",
        params: [shardId, hash, fullTx],
      });
    } catch (error) {
      throw new BlockNotFoundError({
        blockNumberOrHash: hash,
        cause: error,
      });
    }
  }

  /**
   * Returns the block with the given number.
   * @param blockNumber The number of the block whose information is requested.
   * @param fullTx The flag that determines whether full transaction information is returned in the output.
   * @param shardId The ID of the shard where the block was generated.
   * @returns Returns information about a block with the given number.
   * @example
   import { PublicClient } from '@nilfoundation/niljs';
   *
   * const client = new PublicClient({
   *  endpoint: RPC_ENDPOINT
   * })
   *
   * const block = await client.getBlockByNumber(1);
   */
  public async getBlockByNumber(
    blockNumber: Hex,
    fullTx = false,
    shardId = this.shardId,
  ) {
    assertIsValidShardId(shardId);

    try {
      return await this.request<Block<typeof fullTx>>({
        method: "eth_getBlockByNumber",
        params: [shardId, blockNumber, fullTx],
      });
    } catch (error) {
      throw new BlockNotFoundError({
        blockNumberOrHash: blockNumber,
        cause: error,
      });
    }
  }

  /**
   * Returns the total number of messages recorded in the block with the given number.
   * @param number The number of the block whose information is requested.
   * @returns The number of messages contained within the block.
   * @example
   * import { PublicClient } from '@nilfoundation/niljs';
   *
   * const client = new PublicClient({
   *  endpoint: RPC_ENDPOINT
   * })
   *
   * const count = await client.getBlockMessageCountByNumber(1);
   *
   */
  public async getBlockMessageCountByNumber(
    blockNumber: string,
    shardId = this.shardId,
  ) {
    assertIsValidShardId(shardId);

    const res = await this.request<number>({
      method: "eth_getBlockTransactionCountByNumber",
      params: [shardId, blockNumber],
    });

    return res;
  }

  /**
   * Returns the total number of messages recorded in the block with the given hash.
   * @param hash The hash of the block whose information is requested.
   * @param shardId The ID of the shard where the block was generated.
   * @returns The number of messages contained within the block.
   * @example
   * import { PublicClient } from '@nilfoundation/niljs';
   *
   * const client = new PublicClient({
   *  endpoint: RPC_ENDPOINT
   * })
   *
   * const count = await client.getBlockMessageCountByHash(HASH);
   */
  public async getBlockMessageCountByHash(hash: Hex, shardId = this.shardId) {
    assertIsValidShardId(shardId);

    const res = await this.request<number>({
      method: "eth_getBlockTransactionCountByHash",
      params: [shardId, hash],
    });

    return res;
  }

  /**
   * Returns the bytecode of the contract with the given address and at the given block.
   * @param address The address of the account or contract.
   * @param blockNumberOrHash The number/hash of the block.
   * @param shardId The ID of the shard where the block was generated.
   * @returns The bytecode of the contract.
   * @example
   * import { PublicClient } from '@nilfoundation/niljs';
   *
   * const client = new PublicClient({
   *  endpoint: RPC_ENDPOINT
   * })
   *
   * const code = await client.getCode(ADDRESS, 'latest');
   */
  public async getCode(address: IAddress, blockNumberOrHash?: Hex | BlockTag) {
    const res = await this.request<`0x${string}`>({
      method: "eth_getCode",
      params: [address, blockNumberOrHash ?? "latest"],
    });

    return hexToBytes(res);
  }

  /**
   * Returns the transaction count of the account with the given address and at the given block.
   * @param address The address of the account or contract.
   * @param blockNumberOrHash The number/hash of the block.
   * @returns The number of transactions contained within the block.
   * @example
   * import { PublicClient } from '@nilfoundation/niljs';
   *
   * const client = new PublicClient({
   *  endpoint: RPC_ENDPOINT
   * })
   *
   * const count = await client.getMessageCount(ADDRESS, 'latest');
   *
   */
  public async getMessageCount(
    address: IAddress,
    blockNumberOrHash?: Hex | BlockTag,
  ) {
    const res = await this.request<Hex>({
      method: "eth_getTransactionCount",
      params: [address, blockNumberOrHash ?? "latest"],
    });

    return hexToNumber(res);
  }

  /**
   * Returns the balance of the given address and at the given block.
   * @param address The address of the account or contract.
   * @param blockNumberOrHash The number/hash of the block.
   * @returns The balance of the address.
   * @example
   * import { PublicClient } from '@nilfoundation/niljs';
   *
   * const client = new PublicClient({
   *  endpoint: RPC_ENDPOINT
   * })
   *
   * const balance = await client.getBalance(ADDRESS, 'latest');
   */
  public async getBalance(
    address: IAddress,
    blockNumberOrHash?: Hex | BlockTag,
  ) {
    const res = await this.request<`0x${string}`>({
      method: "eth_getBalance",
      params: [addHexPrefix(address), blockNumberOrHash ?? "latest"],
    });

    return hexToBigInt(res);
  }

  /**
   * Returns the structure of the internal message with the given hash.
   * @param hash The hash of the message.
   * @param shardId The ID of the shard where the message was recorded.
   * @returns The message whose information is requested.
   * @example
   * import { PublicClient } from '@nilfoundation/niljs';
   *
   * const client = new PublicClient({
   *  endpoint: RPC_ENDPOINT
   * })
   *
   * const message = await client.getMessageByHash(HASH);
   */
  public async getMessageByHash(
    hash: Hex,
    shardId = this.shardId,
  ): Promise<ProcessedMessage> {
    assertIsValidShardId(shardId);

    const res = await this.request<RPCMessage>({
      method: "eth_getInMessageByHash",
      params: [shardId, hash],
    });

    return {
      ...res,
      value: BigInt(res.value),
      gasLimit: BigInt(res.gasLimit),
      gasUsed: hexToBigInt(res.gasUsed),
      seqno: hexToBigInt(res.seqno),
      index: res.index ? hexToNumber(res.index) : 0,
    };
  }

  /**
   * Returns the receipt for the message with the given hash.
   * @param hash The hash of the message.
   * @param shardId The ID of the shard where the message was recorded.
   * @returns The receipt whose structure is requested.
   * @example
   * import { PublicClient } from '@nilfoundation/niljs';
   *
   * const client = new PublicClient({
   * endpoint: RPC_ENDPOINT
   * })
   *
   * const receipt = await client.getMessageReceiptByHash(1, HASH);
   */
  public async getMessageReceiptByHash(
    hash: Hex,
    shardId = this.shardId,
  ): Promise<ProcessedReceipt | null> {
    assertIsValidShardId(shardId);

    const mapReceipt = (receipt: IReceipt): ProcessedReceipt => {
      return {
        ...receipt,
        gasUsed: BigInt(receipt.gasUsed),
        gasPrice: receipt.gasPrice ? BigInt(receipt.gasPrice) : 0n,
        outputReceipts:
          receipt.outputReceipts?.map((x) => {
            if (x === null) {
              return null;
            }
            return mapReceipt(x);
          }) ?? null,
      };
    };

    const res = await this.request<IReceipt | null>({
      method: "eth_getInMessageReceipt",
      params: [
        shardId,
        typeof hash === "string"
          ? addHexPrefix(hash)
          : addHexPrefix(bytesToHex(hash)),
      ],
    });

    if (res === null) {
      return null;
    }

    return mapReceipt(res);
  }

  /**
   * Creates a new message or creates a contract for a previously signed message.
   * @param message The encoded bytecode of the message.
   * @returns The hash of the message.
   * @example
   * import { PublicClient } from '@nilfoundation/niljs';
   *
   * const client = new PublicClient({
   *  endpoint: RPC_ENDPOINT
   * })
   *
   * const message = Uint8Array.from(ARRAY);
   */
  public async sendRawMessage(message: `0x${string}` | Uint8Array) {
    const res = await this.request<Hex>({
      method: "eth_sendRawTransaction",
      params: [
        typeof message === "string"
          ? message
          : addHexPrefix(bytesToHex(message)),
      ],
    });

    return res;
  }

  /**
   * Returns the gas price in wei.
   * @returns The gas price.
   */
  public async getGasPrice(shardId: number): Promise<bigint> {
    const price = await this.request<`0x${string}`>({
      method: "eth_gasPrice",
      params: [shardId],
    });

    return hexToBigInt(price);
  }

  /**
   * Returns the gas limit.
   * @returns The gas limit.
   */
  public async estimateGasLimit(): Promise<bigint> {
    const stubGasLimit = BigInt(1000000);

    return stubGasLimit;
  }

  /**
   * Returns the chain ID.
   * @returns The chain ID.
   */
  public async chainId(): Promise<number> {
    const res = await this.request<Hex>({
      method: "eth_chainId",
      params: [],
    });

    return hexToNumber(res);
  }

  /**
   * Returns all tokens at the given address.
   * @param address The address whose information is requested.
   * @param blockNumberOrHash The number/hash of the block.
   * @returns The list of tokens.
   */
  public async getCurrencies(
    address: IAddress,
    blockNumberOrHash: Hex | BlockTag,
  ) {
    const res = await this.request<{ [id: string]: `0x${string}` } | null>({
      method: "eth_getCurrencies",
      params: [address, blockNumberOrHash],
    });
    const tokenMap: Record<string, bigint> = {};

    if (res) {
      for (const [key, value] of Object.entries(res)) {
        tokenMap[key] = hexToBigInt(value);
      }
    }

    return tokenMap;
  }

  /**
   * Performs a call to the specified address.
   * @param callArgs The arguments for the call.
   * @param callArgs.from The address of the sender.
   * @param callArgs.to The address of the receiver.
   * @param callArgs.data The data to be sent.
   * @param callArgs.value The value to be sent.
   * @param callArgs.feeCredit The fee credit.
   * @param blockNumberOrHash The number/hash of the block.
   * @param overrides The overrides of state for the chain call.
   */
  public async call(
    callArgs: CallArgs,
    blockNumberOrHash: Hex | BlockTag,
    overrides?: Record<Address, ContractOverride>,
  ) {
    let data: Hex;
    if (callArgs.abi) {
      data = encodeFunctionData({
        abi: callArgs.abi,
        functionName: callArgs.functionName,
        args: callArgs.args || [],
      });
    } else {
      data =
        typeof callArgs.data === "string"
          ? callArgs.data
          : addHexPrefix(bytesToHex(callArgs.data));
    }
    const sendData = {
      from: callArgs.from || undefined,
      to: callArgs.to,
      data: data,
      value: toHex(callArgs.value || 0n),
      feeCredit: (callArgs.feeCredit || 5_000_000n).toString(10),
    };

    const params: unknown[] = [sendData, blockNumberOrHash];
    if (overrides) {
      params.push(overrides);
    }

    const res = await this.request<CallRes>({
      method: "eth_call",
      params,
    });

    if (callArgs.abi) {
      const result = decodeFunctionResult({
        abi: callArgs.abi,
        functionName: callArgs.functionName,
        data: res.data,
      });
      return {
        ...res,
        decodedData: result,
      };
    }

    return res;
  }
}

export { PublicClient };
