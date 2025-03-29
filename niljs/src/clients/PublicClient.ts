import type { Address } from "abitype";
import { decodeFunctionResult, encodeFunctionData } from "viem";
import { bytesToHex } from "../encoding/fromBytes.js";
import { hexToBigInt, hexToBytes, hexToNumber } from "../encoding/fromHex.js";
import { toHex } from "../encoding/toHex.js";
import { BlockNotFoundError } from "../errors/block.js";
import type { IAddress } from "../signers/types/IAddress.js";
import type { Block, BlockTag } from "../types/Block.js";
import type { CallArgs, CallRes, ContractOverride } from "../types/CallArgs.js";
import type { Hex } from "../types/Hex.js";
import type { ProcessedReceipt, Receipt } from "../types/IReceipt.js";
import type { ProcessedTransaction } from "../types/ProcessedTransaction.js";
import type { RPCTransaction } from "../types/RPCTransaction.js";
import { assertIsValidShardId } from "../utils/assert.js";
import { addHexPrefix } from "../utils/hex.js";
import { mapReceipt } from "../utils/receipt.js";
import { BaseClient } from "./BaseClient.js";
import type { IPublicClientConfig } from "./types/Configs.js";
import type { EstimateFeeResult } from "./types/EstimateFeeResult.js";

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
  public async getBlockByHash(hash: Hex, fullTx = false) {
    const block = await this.request<Block<typeof fullTx>>({
      method: "eth_getBlockByHash",
      params: [hash, fullTx],
    });

    if (block === null) {
      throw new BlockNotFoundError({
        blockNumberOrHash: hash,
        docsPath: "/reference/client/classes/PublicClient#getblockbyhash",
      });
    }

    return block;
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
    blockNumber: Hex | BlockTag,
    fullTx = false,
    shardId = this.shardId,
  ) {
    assertIsValidShardId(shardId);

    const block = await this.request<Block<typeof fullTx>>({
      method: "eth_getBlockByNumber",
      params: [shardId, blockNumber, fullTx],
    });

    if (block === null) {
      throw new BlockNotFoundError({
        blockNumberOrHash: blockNumber,
        docsPath: "/reference/client/classes/PublicClient#getblockbynumber",
      });
    }

    return block;
  }

  /**
   * Returns the total number of transactions recorded in the block with the given number.
   * @param number The number of the block whose information is requested.
   * @returns The number of transactions contained within the block.
   * @example
   * import { PublicClient } from '@nilfoundation/niljs';
   *
   * const client = new PublicClient({
   *  endpoint: RPC_ENDPOINT
   * })
   *
   * const count = await client.getBlockTransactionCountByNumber(1);
   *
   */
  public async getBlockTransactionCountByNumber(
    blockNumber: Hex | BlockTag,
    shardId = this.shardId,
  ) {
    assertIsValidShardId(shardId);

    return await this.request<number>({
      method: "eth_getBlockTransactionCountByNumber",
      params: [shardId, blockNumber],
    });
  }

  /**
   * Returns the total number of transactions recorded in the block with the given hash.
   * @param hash The hash of the block whose information is requested.
   * @returns The number of transactions contained within the block.
   * @example
   * import { PublicClient } from '@nilfoundation/niljs';
   *
   * const client = new PublicClient({
   *  endpoint: RPC_ENDPOINT
   * })
   *
   * const count = await client.getBlockTransactionCountByHash(HASH);
   */
  public async getBlockTransactionCountByHash(hash: Hex) {
    return await this.request<number>({
      method: "eth_getBlockTransactionCountByHash",
      params: [hash],
    });
  }

  /**
   * Returns the bytecode of the contract with the given address and at the given block.
   * @param address The address of the account or contract.
   * @param blockNumberOrHash The number/hash of the block.
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
   * const count = await client.getTransactionCount(ADDRESS, 'latest');
   *
   */
  public async getTransactionCount(address: IAddress, blockNumberOrHash?: Hex | BlockTag) {
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
  public async getBalance(address: IAddress, blockNumberOrHash?: Hex | BlockTag) {
    const res = await this.request<`0x${string}`>({
      method: "eth_getBalance",
      params: [addHexPrefix(address), blockNumberOrHash ?? "latest"],
    });

    return hexToBigInt(res);
  }

  /**
   * Returns the structure of the internal transaction with the given hash.
   * @param hash The hash of the transaction.
   * @returns The transaction whose information is requested.
   * @example
   * import { PublicClient } from '@nilfoundation/niljs';
   *
   * const client = new PublicClient({
   *  endpoint: RPC_ENDPOINT
   * })
   *
   * const transaction = await client.getTransactionByHash(HASH);
   */
  public async getTransactionByHash(hash: Hex): Promise<ProcessedTransaction> {
    const res = await this.request<RPCTransaction>({
      method: "eth_getInTransactionByHash",
      params: [hash],
    });

    return {
      ...res,
      value: BigInt(res.value),
      feeCredit: BigInt(res.feeCredit || 0),
      maxPriorityFeePerGas: BigInt(res.maxPriorityFeePerGas || 0),
      maxFeePerGas: BigInt(res.maxFeePerGas || 0),
      gasUsed: hexToBigInt(res.gasUsed),
      seqno: hexToBigInt(res.seqno),
      index: res.index ? hexToNumber(res.index) : 0,
    };
  }

  /**
   * Returns the receipt for the transaction with the given hash.
   * @param hash The hash of the transaction.
   * @returns The receipt whose structure is requested.
   * @example
   * import { PublicClient } from '@nilfoundation/niljs';
   *
   * const client = new PublicClient({
   * endpoint: RPC_ENDPOINT
   * })
   *
   * const receipt = await client.getTransactionReceiptByHash(HASH, 1);
   */
  public async getTransactionReceiptByHash(hash: Hex): Promise<ProcessedReceipt | null> {
    const res = await this.request<Receipt | null>({
      method: "eth_getInTransactionReceipt",
      params: [typeof hash === "string" ? addHexPrefix(hash) : addHexPrefix(bytesToHex(hash))],
    });

    if (res === null) {
      return null;
    }

    return mapReceipt(res);
  }

  /**
   * Creates a new transaction or creates a contract for a previously signed transaction.
   * @param transaction The encoded bytecode of the transaction.
   * @returns The hash of the transaction.
   * @example
   * import { PublicClient } from '@nilfoundation/niljs';
   *
   * const client = new PublicClient({
   *  endpoint: RPC_ENDPOINT
   * })
   *
   * const transaction = Uint8Array.from(ARRAY);
   */
  public async sendRawTransaction(transaction: `0x${string}` | Uint8Array) {
    const res = await this.request<Hex>({
      method: "eth_sendRawTransaction",
      params: [
        typeof transaction === "string" ? transaction : addHexPrefix(bytesToHex(transaction)),
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

  private chainIdCache: number | null = null;

  /**
   * Returns the chain ID.
   * @returns The chain ID.
   */
  public async chainId(): Promise<number> {
    if (this.chainIdCache) {
      return this.chainIdCache;
    }

    const res = await this.request<Hex>({
      method: "eth_chainId",
      params: [],
    });

    this.chainIdCache = hexToNumber(res);
    return hexToNumber(res);
  }

  public clearChainIdCache() {
    this.chainIdCache = null;
  }

  /**
   * Returns all tokens at the given address.
   * @param address The address whose information is requested.
   * @param blockNumberOrHash The number/hash of the block.
   * @returns The list of tokens.
   */
  public async getTokens(address: IAddress, blockNumberOrHash: Hex | BlockTag) {
    const res = await this.request<{ [id: string]: `0x${string}` } | null>({
      method: "eth_getTokens",
      params: [address, blockNumberOrHash],
    });
    const tokenMap: Record<string, bigint> = {};

    if (res) {
      for (const [key, value] of Object.entries(res)) {
        tokenMap[key] = BigInt(value);
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
        typeof callArgs.data === "string" ? callArgs.data : addHexPrefix(bytesToHex(callArgs.data));
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

  /**
   * Performs a call to the specified address.
   * @param callArgs The arguments for the call.
   * @param callArgs.from The address of the sender.
   * @param callArgs.to The address of the receiver.
   * @param callArgs.data The data to be sent.
   * @param callArgs.value The value to be sent.
   * @param callArgs.feeCredit The fee credit.
   * @param blockNumberOrHash The number/hash of the block.
   */
  public async estimateGas(
    callArgs: CallArgs,
    blockNumberOrHash: Hex | BlockTag,
  ): Promise<EstimateFeeResult> {
    let data: Hex;
    if (callArgs.abi) {
      data = encodeFunctionData({
        abi: callArgs.abi,
        functionName: callArgs.functionName,
        args: callArgs.args || [],
      });
    } else {
      data =
        typeof callArgs.data === "string" ? callArgs.data : addHexPrefix(bytesToHex(callArgs.data));
    }
    const sendData = {
      flags: callArgs.flags || [""],
      from: callArgs.from || undefined,
      to: callArgs.to,
      data: data,
      value: toHex(callArgs.value || 0n),
      feeCredit: (callArgs.feeCredit || 0).toString(10),
    };

    const params: unknown[] = [sendData, blockNumberOrHash];

    const resStr = await this.request<{
      feeCredit: Hex;
      averagePriorityFee: Hex;
      maxBaseFee: Hex;
    }>({
      method: "eth_estimateFee",
      params,
    });

    const res = {
      feeCredit: BigInt(resStr.feeCredit),
      averagePriorityFee: BigInt(resStr.averagePriorityFee),
      maxBaseFee: BigInt(resStr.maxBaseFee),
    };

    return res;
  }

  /**
   * Returns the list of shard IDs.
   * @returns The list of shard IDs.
   */
  public async getShardIdList() {
    const res = await this.request<number[]>({
      method: "eth_getShardIdList",
      params: [],
    });

    return res;
  }

  /**
   * Returns the number of shards.
   * @returns The number of shards.
   */
  public async getNumShards() {
    const res = await this.request<number>({
      method: "eth_getNumShards",
      params: [],
    });

    return res;
  }
}

export { PublicClient };
