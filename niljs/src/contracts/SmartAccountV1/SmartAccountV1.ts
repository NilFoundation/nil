import { SmartAccount } from "@nilfoundation/smart-contracts";
import type { Abi } from "abitype";
import invariant from "tiny-invariant";
import { bytesToHex, encodeDeployData, encodeFunctionData } from "viem";
import type { PublicClient } from "../../clients/PublicClient.js";
import type { ContractFunctionName } from "../../contract-factory/ContractFactory.js";
import { prepareDeployPart } from "../../encoding/deployPart.js";
import { externalTransactionEncode } from "../../encoding/externalTransaction.js";
import { hexToBytes } from "../../encoding/fromHex.js";
import type { ISigner } from "../../signers/index.js";
import type {
  SendTransactionParams,
  SmartAccountInterface,
} from "../../smart-accounts/SmartAccountInterface.js";
import type { Hex } from "../../types/Hex.js";
import type { IDeployData } from "../../types/IDeployData.js";
import { calculateAddress, getShardIdFromAddress, refineAddress } from "../../utils/address.js";
import { addHexPrefix } from "../../utils/hex.js";
import {
  refineBigintSalt,
  refineCompressedPublicKey,
  refineFunctionHexData,
  refineSalt,
} from "../../utils/refiners.js";
import type {
  DeployParams,
  RequestParams,
  SendSyncTransactionParams,
  SmartAccountV1Config,
} from "./types/index.js";

/**
 * SmartAccountV1 is a class used for performing operations on the cluster that require authentication.
 *
 * @class SmartAccountV1
 * @typedef {SmartAccountV1}
 */
export class SmartAccountV1 implements SmartAccountInterface {
  /**
   * The smart account bytecode.
   *
   * @static
   * @type {*}
   */
  static code = hexToBytes(addHexPrefix(SmartAccount.bytecode));
  /**
   * The smart account ABI.
   *
   * @static
   * @type {Abi}
   */
  static abi = SmartAccount.abi;

  /**
   * Calculates the address of the new smart account.
   *
   * @static
   * @param {{
   *     pubKey: Uint8Array;
   *     shardId: number;
   *     salt: Uint8Array | bigint;
   *   }} param0 The object representing the config for address calculation.
   * @param {Uint8Array} param0.pubKey The smart account public key.
   * @param {number} param0.shardId The ID of the shard where the smart account should be deployed.
   * @param {Uint8Array | bigint} param0.salt Arbitrary data change the address.
   * @returns {Uint8Array} The address of the new smart account.
   * @example
   * import {
       LocalECDSAKeySigner,
       SmartAccountV1,
       generateRandomPrivateKey,
     } from '@nilfoundation/niljs';

   * const signer = new LocalECDSAKeySigner({
       privateKey: generateRandomPrivateKey(),
     });

     const pubkey = signer.getPublicKey();

   * const anotherAddress = SmartAccountV1.calculateSmartAccountAddress({
       pubKey: pubkey,
       shardId: 1,
       salt: 200n,
     });
   */
  static calculateSmartAccountAddress({
    pubKey,
    shardId,
    salt,
  }: {
    pubKey: Uint8Array;
    shardId: number;
    salt: Uint8Array | bigint;
  }) {
    const { address } = prepareDeployPart({
      abi: SmartAccount.abi as Abi,
      bytecode: SmartAccountV1.code,
      args: [bytesToHex(pubKey)],
      salt: salt,
      shard: shardId,
    });

    return refineAddress(address);
  }

  /**
   * The smart account public key.
   *
   * @type {Uint8Array}
   */
  pubkey: Uint8Array;
  /**
   * The ID of the shard where the smart account is deployed.
   *
   * @type {number}
   */
  shardId: number;
  /**
   * The client for interacting with the smart account.
   *
   * @type {PublicClient}
   */
  client: PublicClient;
  /**
   * Arbitrary data for changing the smart account address.
   *
   * @type {Uint8Array}
   */
  salt?: Uint8Array;
  /**
   * The smart account signer.
   *
   * @type {ISigner}
   */
  signer: ISigner;
  /**
   * The smart account address.
   *
   * @type {Hex}
   */
  address: Hex;

  /**
   * Creates an instance of SmartAccountV1.
   *
   * @constructor
   * @param {SmartAccountV1Config} param0 The object representing the initial smart account config. See {@link SmartAccountV1Config}.
   * @param {SmartAccountV1Config} param0.pubkey The smart account public key.
   * @param {SmartAccountV1Config} param0.shardId The ID of the shard where the smart account is deployed.
   * @param {SmartAccountV1Config} param0.address The smart account address. If address is not provided it will be calculated with salt.
   * @param {SmartAccountV1Config} param0.client The client for interacting with the smart account.
   * @param {SmartAccountV1Config} param0.salt The arbitrary data for changing the smart account address.
   * @param {SmartAccountV1Config} param0.signer The smart account signer.
   */
  constructor({ pubkey, shardId, address, client, salt, signer }: SmartAccountV1Config) {
    this.pubkey = refineCompressedPublicKey(pubkey);
    this.client = client;
    this.signer = signer;
    invariant(
      !(salt && address),
      "You should use salt and shard for calculating address or address itself, not both to avoid issue.",
    );
    this.address = address
      ? refineAddress(address)
      : SmartAccountV1.calculateSmartAccountAddress({ pubKey: this.pubkey, shardId, salt });
    if (salt !== undefined) {
      this.salt = refineSalt(salt);
    }
    this.shardId = getShardIdFromAddress(this.address);
  }

  /**
   * Deploys the smart account.
   *
   * @async
   * @param {boolean} [waitTillConfirmation=true] The flag that determines whether the function waits for deployment confirmation before exiting.
   * @param {bigint} [feeCredit] The fee credit for processing the deployment transaction. If not set, it will be estimated automatically.
   * @returns {Uint8Array} The hash of the deployment transaction.
   * @example
   * import {
       Faucet,
       HttpTransport,
       LocalECDSAKeySigner,
       PublicClient,
       SmartAccountV1,
       generateRandomPrivateKey,
     } from '@nilfoundation/niljs';
   * const client = new PublicClient({
       transport: new HttpTransport({
         endpoint: RPC_ENDPOINT,
       }),
       shardId: 1,
     });
   * const signer = new LocalECDSAKeySigner({
       privateKey: generateRandomPrivateKey(),
     });
   * const faucet = new Faucet(client);
   * await faucet.withdrawTo(smartAccountAddress, 100000n);
   * const pubkey = signer.getPublicKey();
   * const smartAccount = new SmartAccountV1({
       pubkey: pubkey,
       salt: 100n,
       shardId: 1,
       client,
       signer,
       address: SmartAccountV1.calculateSmartAccountAddress({
         pubKey: pubkey,
         shardId: 1,
         salt: 100n,
       }),
     });
   * await smartAccount.selfDeploy(true);
   */
  async selfDeploy(waitTillConfirmation = true, feeCredit?: bigint) {
    invariant(
      typeof this.salt !== "undefined",
      "Salt is required for external deployment. Please provide salt for walelt",
    );

    const [balance, code] = await Promise.all([
      await this.client.getBalance(this.address, "latest"),
      await this.client.getCode(this.address, "latest").catch(() => Uint8Array.from([])),
    ]);

    invariant(code.length === 0, "Contract already deployed");
    invariant(balance > 0n, "Insufficient balance");

    const { data } = prepareDeployPart({
      abi: SmartAccount.abi as Abi,
      bytecode: SmartAccountV1.code,
      args: [bytesToHex(this.pubkey)],
      salt: this.salt,
      shard: this.shardId,
    });

    let refinedCredit = feeCredit;

    if (!refinedCredit) {
      const { raw } = await this.requestToSmartAccount(
        {
          data: data,
          deploy: true,
          seqno: 0,
        },
        false,
      );

      refinedCredit = await this.client.estimateGas(
        {
          to: this.address,
          data: raw,
        },
        "latest",
      );
    }

    const { hash } = await this.requestToSmartAccount({
      data: data,
      deploy: true,
      seqno: 0,
      feeCredit: refinedCredit,
    });

    if (waitTillConfirmation) {
      while (true) {
        const code = await this.client.getCode(this.address, "latest");
        if (code.length > 0) {
          break;
        }
        await new Promise((resolve) => setTimeout(resolve, 1000));
      }
    }
    return hash;
  }

  /**
   * Checks the deployment status.
   *
   * @async
   * @returns {Promise<boolean>} The current deployment status.
   */
  async checkDeploymentStatus(): Promise<boolean> {
    const code = await this.client.getCode(this.address, "latest");
    return code.length > 0;
  }

  /**
   * Performs a request to the smart account.
   *
   * @async
   * @param {RequestParams} requestParams The object representing the request parameters.
   * @param {boolean} [send=true] The flag that determines whether the request is sent when the function is called.
   * @returns {Promise<{ raw: Uint8Array; hash: Uint8Array, seqno: number, chainId: number }>} The transaction bytecode and hash.
   */
  async requestToSmartAccount(
    requestParams: RequestParams,
    send = true,
  ): Promise<{ raw: Uint8Array; hash: Uint8Array; seqno: number; chainId: number }> {
    const [seqno, chainId] = await Promise.all([
      requestParams.seqno ?? this.client.getTransactionCount(this.address, "latest"),
      requestParams.chainId ?? this.client.chainId(),
    ]);
    const encodedTransaction = await externalTransactionEncode(
      {
        isDeploy: requestParams.deploy,
        to: hexToBytes(this.address),
        chainId: chainId,
        seqno,
        data: requestParams.data,
        feeCredit: requestParams.feeCredit ?? 5_000_000n,
      },
      this.signer,
    );
    if (send) await this.client.sendRawTransaction(encodedTransaction.raw);
    return { ...encodedTransaction, seqno, chainId };
  }

  /**
   * Send a transaction via the smart account.
   *
   * @async
   * @param {SendTransactionParams} param0 The object representing the transaction params.
   * @param {SendTransactionParams} param0.to The address where the transaction should be sent.
   * @param {SendTransactionParams} param0.refundTo The address where the gas cost should be refunded.
   * @param {SendTransactionParams} param0.bounceTo The address where the transaction value should be refunded in case of failure.
   * @param {SendTransactionParams} param0.tokens The tokens to be sent with the transaction.
   * @param {SendTransactionParams} param0.data The transaction bytecode.
   * @param {SendTransactionParams} param0.abi The transaction abi for encoding.
   * @param {SendTransactionParams} param0.functionName The transaction function name for abi.
   * @param {SendTransactionParams} param0.args The transaction args name for abi.
   * @param {SendTransactionParams} param0.deploy The flag that determines whether the transaction is a deploy transaction.
   * @param {SendTransactionParams} param0.seqno The transaction sequence number.
   * @param {SendTransactionParams} param0.feeCredit The transaction fee credit for processing transaction on receiving shard.
   * @param {SendTransactionParams} param0.value The transaction value.
   * @param {SendTransactionParams} param0.chainId The transaction chain id.
   * @returns {unknown} The transaction hash.
   * @example
   * const anotherAddress = SmartAccountV1.calculateSmartAccountAddress({
   *     pubKey: pubkey,
   *     shardId: 1,
   *     salt: 200n,
   *   });
   * await smartAccount.sendTransaction({
   *     to: anotherAddress,
   *     value: 10n,
   *     gas: 100000n,
   *   });
   */
  async sendTransaction({
    to,
    refundTo,
    bounceTo,
    data,
    abi,
    functionName,
    args,
    seqno,
    feeCredit,
    value,
    tokens,
    chainId,
  }: SendTransactionParams) {
    const hexTo = refineAddress(to);
    const hexRefundTo = refineAddress(refundTo ?? this.address);
    const hexBounceTo = refineAddress(bounceTo ?? this.address);
    const hexData = refineFunctionHexData({ data, abi, functionName, args });
    let refinedCredit = feeCredit;

    const callData = encodeFunctionData({
      abi: SmartAccount.abi,
      functionName: "asyncCall",
      args: [hexTo, hexRefundTo, hexBounceTo, tokens ?? [], value ?? 0n, hexData],
    });

    if (!refinedCredit) {
      const balance = await this.getBalance();

      refinedCredit = await this.client.estimateGas(
        {
          to: this.address,
          from: this.address,
          data: hexToBytes(callData),
        },
        "latest",
      );

      if (refinedCredit > balance) {
        throw new Error("Insufficient balance");
      }
    }

    const { hash } = await this.requestToSmartAccount({
      data: hexToBytes(callData),
      deploy: false,
      seqno: seqno,
      chainId: chainId,
      feeCredit: refinedCredit,
    });

    return bytesToHex(hash);
  }

  /**
   * Sets the name of the custom token that the smart account can own and mint.
   *
   * @async
   * @param {string} The name of the custom token.
   * @returns {unknown} The transaction hash.
   * @example
   * const hashTransaction = await smartAccount.setTokenName("MY_TOKEN");
   * await waitTillCompleted(client, hashTransaction);
   */
  async setTokenName(name: string) {
    const callData = encodeFunctionData({
      abi: SmartAccount.abi,
      functionName: "setTokenName",
      args: [name],
    });

    const { hash } = await this.requestToSmartAccount({
      data: hexToBytes(callData),
      deploy: false,
    });

    return bytesToHex(hash);
  }

  /**
   * Mints the token that the smart account owns and withdraws it to the smart account.
   * {@link setTokenName} has to be called first before minting a token.
   *
   * @async
   * @param {bigint} The amount to mint.
   * @returns {unknown} The transaction hash.
   * @example
   * const hashTransaction = await smartAccount.mintToken(mintCount);
   * await waitTillCompleted(client, hashTransaction);
   */
  async mintToken(amount: bigint) {
    return await this.changeTokenAmount(amount, true);
  }

  /**
   * Burns the token that the smart account owns.
   *
   * @async
   * @param {bigint} The amount to burn.
   * @returns {unknown} The transaction hash.
   * @example
   * const hashTransaction = await smartAccount.burnToken(burnToken);
   * await waitTillCompleted(client, hashTransaction);
   */
  async burnToken(amount: bigint) {
    return await this.changeTokenAmount(amount, false);
  }

  private async changeTokenAmount(amount: bigint, mint: boolean) {
    let method: ContractFunctionName<typeof SmartAccount.abi> = "burnToken" as const;
    if (mint) {
      method = "mintToken" as const;
    }

    const callData = encodeFunctionData({
      abi: SmartAccount.abi,
      functionName: method,
      args: [amount],
    });

    const { hash } = await this.requestToSmartAccount({
      data: hexToBytes(callData),
      deploy: false,
    });

    return bytesToHex(hash);
  }

  /**
   * Send a raw signed transaction via the smart account.
   *
   * @async
   * @param {Uint8Array} rawTransaction The transaction bytecode.
   * @returns {unknown} The transaction hash.
   */
  async sendRawInternalTransaction(rawTransaction: Uint8Array) {
    const { hash } = await this.requestToSmartAccount({
      data: rawTransaction,
      deploy: false,
    });

    return bytesToHex(hash);
  }

  /**
   * Deploys a new smart contract via the smart account.
   *
   * @async
   * @param {DeployParams} param0 The object representing the contract deployment params.
   * @param {DeployParams} param0.shardId The ID of the shard where the contract should be deployed.
   * @param {DeployParams} param0.bytecode The contract bytecode.
   * @param {DeployParams} param0.abi The contract ABI.
   * @param {DeployParams} param0.args The arbitrary arguments for deployment.
   * @param {DeployParams} param0.salt The arbitrary data for changing the contract address.
   * @param {DeployParams} param0.value The deployment transaction value.
   * @param {DeployParams} param0.feeCredit The deployment transaction fee credit.
   * @param {DeployParams} param0.seqno The deployment transaction seqno.
   * @param {DeployParams} param0.chainId The deployment transaction chain id.
   * @returns {unknown} The object containing the deployment transaction hash and the contract address.
   */
  async deployContract({
    shardId,
    bytecode,
    abi,
    args,
    salt,
    value,
    feeCredit,
    seqno,
    chainId,
  }: DeployParams) {
    let deployData: IDeployData;
    if (abi && args) {
      deployData = {
        shard: shardId,
        bytecode,
        abi: abi,
        args: args,
        salt,
      };
    } else {
      invariant(!(abi || args), "ABI and args should be provided together or not provided at all.");
      deployData = {
        shard: shardId,
        bytecode,
        salt,
      };
    }

    let constructorData: Uint8Array;
    if (abi) {
      constructorData = hexToBytes(
        encodeDeployData({
          abi: abi,
          bytecode:
            typeof deployData.bytecode === "string"
              ? deployData.bytecode
              : bytesToHex(deployData.bytecode),
          args: deployData.args || [],
        }),
      );
    } else {
      constructorData =
        typeof deployData.bytecode === "string"
          ? hexToBytes(deployData.bytecode)
          : deployData.bytecode;
    }
    const address = calculateAddress(
      deployData.shard,
      constructorData,
      refineSalt(deployData.salt),
    );

    const hexData = bytesToHex(constructorData);

    const callData = encodeFunctionData({
      abi: SmartAccount.abi,
      functionName: "asyncDeploy",
      args: [BigInt(deployData.shard), value ?? 0n, hexData, refineBigintSalt(deployData.salt)],
    });

    let refinedCredit = feeCredit;

    if (!refinedCredit) {
      const balance = await this.getBalance();

      refinedCredit = await this.client.estimateGas(
        {
          to: this.address,
          from: this.address,
          data: hexToBytes(callData),
        },
        "latest",
      );
      if (refinedCredit > balance) {
        throw new Error("Insufficient balance");
      }
    }

    const { hash } = await this.requestToSmartAccount({
      data: hexToBytes(callData),
      deploy: false,
      seqno,
      chainId,
      feeCredit: refinedCredit,
    });

    return {
      hash: bytesToHex(hash),
      address: bytesToHex(address),
    };
  }

  /**
   * Creates a new transaction and performs a synchronous call to the specified address.
   *
   * @async
   * @param {SendSyncTransactionParams} param0 The object representing the transaction params.
   * @param {SendSyncTransactionParams} param0.to The address where the transaction should be sent.
   * @param {SendSyncTransactionParams} param0.data The transaction bytecode.
   * @param {SendSyncTransactionParams} param0.abi The transaction abi.
   * @param {SendSyncTransactionParams} param0.functionName The transaction function name for abi.
   * @param {SendSyncTransactionParams} param0.args The transaction args for abi.
   * @param {SendTransactionParams} param0.seqno The transaction sequence number.
   * @param {SendTransactionParams} param0.gas The transaction gas.
   * @param {SendTransactionParams} param0.value The transaction value.
   * @returns {unknown} The transaction hash.
   * @example
   * const anotherAddress = SmartAccountV1.calculateSmartAccountAddress({
   *     pubKey: pubkey,
   *     shardId: 1,
   *     salt: 200n,
   *   });
   * await smartAccount.sendTransaction({
   *     to: anotherAddress,
   *     value: 10n,
   *     gas: 100000n,
   *   });
   */
  async syncSendTransaction({
    to,
    data,
    abi,
    functionName,
    args,
    seqno,
    gas,
    value,
  }: SendSyncTransactionParams) {
    const hexTo = refineAddress(to);
    const hexData = refineFunctionHexData({ data, abi, functionName, args });

    const callData = encodeFunctionData({
      abi: SmartAccount.abi,
      functionName: "syncCall",
      args: [hexTo, gas, value, hexData],
    });

    const { hash } = await this.requestToSmartAccount({
      data: hexToBytes(callData),
      deploy: false,
      seqno,
    });

    return bytesToHex(hash);
  }

  /**
   * Returns the smart account balance.
   *
   * @async
   * @returns {unknown} The smart account balance.
   */
  async getBalance() {
    return this.client.getBalance(this.address, "latest");
  }
}
