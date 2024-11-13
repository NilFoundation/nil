import Wallet from "@nilfoundation/smart-contracts/artifacts/Wallet.json";
import type { Abi } from "abitype";
import invariant from "tiny-invariant";
import { bytesToHex, encodeDeployData, encodeFunctionData } from "viem";
import type { PublicClient } from "../../clients/PublicClient.js";
import { prepareDeployPart } from "../../encoding/deployPart.js";
import { externalMessageEncode } from "../../encoding/externalMessage.js";
import { hexToBytes } from "../../encoding/fromHex.js";
import { addHexPrefix } from "../../index.js";
import type { ISigner } from "../../signers/index.js";
import type { Hex } from "../../types/Hex.js";
import type { IDeployData } from "../../types/IDeployData.js";
import { calculateAddress, getShardIdFromAddress, refineAddress } from "../../utils/address.js";
import {
  refineCompressedPublicKey,
  refineFunctionHexData,
  refineSalt,
} from "../../utils/refiners.js";
import type { SendMessageParams, WalletInterface } from "../../wallets/WalletInterface.js";
import type {
  DeployParams,
  RequestParams,
  SendSyncMessageParams,
  WalletV1Config,
} from "./types/index.js";

/**
 * WalletV1 is a class used for performing operations on the cluster that require authentication.
 *
 * @class WalletV1
 * @typedef {WalletV1}
 */
export class WalletV1 implements WalletInterface {
  /**
   * The wallet bytecode.
   *
   * @static
   * @type {*}
   */
  static code = hexToBytes(addHexPrefix(Wallet.evm.bytecode.object));
  /**
   * The wallet ABI.
   *
   * @static
   * @type {Abi}
   */
  static abi = Wallet.abi as Abi;

  /**
   * Calculates the address of the new wallet.
   *
   * @static
   * @param {{
   *     pubKey: Uint8Array;
   *     shardId: number;
   *     salt: Uint8Array | bigint;
   *   }} param0 The object representing the config for address calculation.
   * @param {Uint8Array} param0.pubKey The wallet public key.
   * @param {number} param0.shardId The ID of the shard where the wallet should be deployed.
   * @param {Uint8Array | bigint} param0.salt Arbitrary data change the address.
   * @returns {Uint8Array} The address of the new wallet.
   * @example
   * import {
       LocalECDSAKeySigner,
       WalletV1,
       generateRandomPrivateKey,
     } from '@nilfoundation/niljs';

   * const signer = new LocalECDSAKeySigner({
       privateKey: generateRandomPrivateKey(),
     });

     const pubkey = signer.getPublicKey();

   * const anotherAddress = WalletV1.calculateWalletAddress({
       pubKey: pubkey,
       shardId: 1,
       salt: 200n,
     });
   */
  static calculateWalletAddress({
    pubKey,
    shardId,
    salt,
  }: {
    pubKey: Uint8Array;
    shardId: number;
    salt: Uint8Array | bigint;
  }) {
    const { address } = prepareDeployPart({
      abi: Wallet.abi as Abi,
      bytecode: WalletV1.code,
      args: [bytesToHex(pubKey)],
      salt: salt,
      shard: shardId,
    });

    return refineAddress(address);
  }

  /**
   * The wallet public key.
   *
   * @type {Uint8Array}
   */
  pubkey: Uint8Array;
  /**
   * The ID of the shard where the wallet is deployed.
   *
   * @type {number}
   */
  shardId: number;
  /**
   * The client for interacting with the wallet.
   *
   * @type {PublicClient}
   */
  client: PublicClient;
  /**
   * Arbitrary data for changing the wallet address.
   *
   * @type {Uint8Array}
   */
  salt?: Uint8Array;
  /**
   * The wallet signer.
   *
   * @type {ISigner}
   */
  signer: ISigner;
  /**
   * The wallet address.
   *
   * @type {Hex}
   */
  address: Hex;

  /**
   * Creates an instance of WalletV1.
   *
   * @constructor
   * @param {WalletV1Config} param0 The object representing the initial wallet config. See {@link WalletV1Config}.
   * @param {WalletV1Config} param0.pubkey The wallet public key.
   * @param {WalletV1Config} param0.shardId The ID of the shard where the wallet is deployed.
   * @param {WalletV1Config} param0.address The wallet address. If address is not provided it will be calculated with salt.
   * @param {WalletV1Config} param0.client The client for interacting with the wallet.
   * @param {WalletV1Config} param0.salt The arbitrary data for changing the wallet address.
   * @param {WalletV1Config} param0.signer The wallet signer.
   */
  constructor({ pubkey, shardId, address, client, salt, signer }: WalletV1Config) {
    this.pubkey = refineCompressedPublicKey(pubkey);
    this.client = client;
    this.signer = signer;
    invariant(
      !(salt && address),
      "You should use salt and shard for calculating address or address itself, not both to avoid issue.",
    );
    this.address = address
      ? refineAddress(address)
      : WalletV1.calculateWalletAddress({ pubKey: this.pubkey, shardId, salt });
    if (salt) {
      this.salt = refineSalt(salt);
    }
    this.shardId = getShardIdFromAddress(this.address);
  }

  /**
   * Deploys the wallet.
   *
   * @async
   * @param {boolean} [waitTillConfirmation=true] The flag that determines whether the function waits for deployment confirmation before exiting.
   * @returns {Uint8Array} The hash of the deployment transaction.
   * @example
   * import {
       Faucet,
       HttpTransport,
       LocalECDSAKeySigner,
       PublicClient,
       WalletV1,
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
   * await faucet.withdrawTo(walletAddress, 100000n);
   * const pubkey = signer.getPublicKey();
   * const wallet = new WalletV1({
       pubkey: pubkey,
       salt: 100n,
       shardId: 1,
       client,
       signer,
       address: WalletV1.calculateWalletAddress({
         pubKey: pubkey,
         shardId: 1,
         salt: 100n,
       }),
     });
   * await wallet.selfDeploy(true);
   */
  async selfDeploy(waitTillConfirmation = true) {
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
      abi: Wallet.abi as Abi,
      bytecode: WalletV1.code,
      args: [bytesToHex(this.pubkey)],
      salt: this.salt,
      shard: this.shardId,
    });

    const { hash } = await this.requestToWallet({
      data: data,
      deploy: true,
      seqno: 0,
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
   * Performs a request to the wallet.
   *
   * @async
   * @param {RequestParams} requestParams The object representing the request parameters.
   * @param {boolean} [send=true] The flag that determines whether the request is sent when the function is called.
   * @returns {Promise<{ raw: Uint8Array; hash: Uint8Array, seqno: number, chainId: number }>} The message bytecode and hash.
   */
  async requestToWallet(
    requestParams: RequestParams,
    send = true,
  ): Promise<{ raw: Uint8Array; hash: Uint8Array; seqno: number; chainId: number }> {
    const [seqno, chainId] = await Promise.all([
      requestParams.seqno ?? this.client.getMessageCount(this.address, "latest"),
      requestParams.chainId ?? this.client.chainId(),
    ]);
    const encodedMessage = await externalMessageEncode(
      {
        isDeploy: requestParams.deploy,
        to: hexToBytes(this.address),
        chainId: chainId,
        seqno,
        data: requestParams.data,
      },
      this.signer,
    );
    if (send) await this.client.sendRawMessage(encodedMessage.raw);
    return { ...encodedMessage, seqno, chainId };
  }

  /**
   * Send a message via the wallet.
   *
   * @async
   * @param {SendMessageParams} param0 The object representing the message params.
   * @param {SendMessageParams} param0.to The address where the message should be sent.
   * @param {SendMessageParams} param0.refundTo The address where the gas cost should be refunded.
   * @param {SendMessageParams} param0.bounceTo The address where the message value should be refunded in case of failure.
   * @param {SendMessageParams} param0.tokens The tokens to be sent with the message.
   * @param {SendMessageParams} param0.data The message bytecode.
   * @param {SendMessageParams} param0.abi The message abi for encoding.
   * @param {SendMessageParams} param0.functionName The message function name for abi.
   * @param {SendMessageParams} param0.args The message args name for abi.
   * @param {SendMessageParams} param0.deploy The flag that determines whether the message is a deploy message.
   * @param {SendMessageParams} param0.seqno The message sequence number.
   * @param {SendMessageParams} param0.feeCredit The message fee credit for processing message on receiving shard.
   * @param {SendMessageParams} param0.value The message value.
   * @param {SendMessageParams} param0.chainId The message chain id.
   * @returns {unknown} The message hash.
   * @example
   * const anotherAddress = WalletV1.calculateWalletAddress({
   *     pubKey: pubkey,
   *     shardId: 1,
   *     salt: 200n,
   *   });
   * await wallet.sendMessage({
   *     to: anotherAddress,
   *     value: 10n,
   *     gas: 100000n,
   *   });
   */
  async sendMessage({
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
  }: SendMessageParams) {
    const hexTo = refineAddress(to);
    const hexRefundTo = refineAddress(refundTo ?? this.address);
    const hexBounceTo = refineAddress(bounceTo ?? this.address);
    const hexData = refineFunctionHexData({ data, abi, functionName, args });

    const callData = encodeFunctionData({
      abi: Wallet.abi,
      functionName: "asyncCall",
      args: [hexTo, hexRefundTo, hexBounceTo, feeCredit ?? 0n, tokens ?? [], value ?? 0n, hexData],
    });

    const { hash } = await this.requestToWallet({
      data: hexToBytes(callData),
      deploy: false,
      seqno: seqno,
      chainId: chainId,
    });

    return bytesToHex(hash);
  }

  /**
   * Sets the name of the custom currency that the wallet can own and mint.
   *
   * @async
   * @param {string} The name of the custom currency.
   * @returns {unknown} The message hash.
   * @example
   * const hashMessage = await wallet.setCurrencyName("MY_TOKEN");
   * await waitTillCompleted(client, hashMessage);
   */
  async setCurrencyName(name: string) {
    const callData = encodeFunctionData({
      abi: Wallet.abi,
      functionName: "setCurrencyName",
      args: [name],
    });

    const { hash } = await this.requestToWallet({
      data: hexToBytes(callData),
      deploy: false,
    });

    return bytesToHex(hash);
  }

  /**
   * Mints the currency that the wallet owns and withdraws it to the wallet.
   * {@link setCurrencyName} has to be called first before minting a currency.
   *
   * @async
   * @param {bigint} The amount to mint.
   * @returns {unknown} The message hash.
   * @example
   * const hashMessage = await wallet.mintCurrency(mintCount);
   * await waitTillCompleted(client, hashMessage);
   */
  async mintCurrency(amount: bigint) {
    return await this.changeCurrencyAmount(amount, true);
  }

  /**
   * Burns the currency that the wallet owns.
   *
   * @async
   * @param {bigint} The amount to burn.
   * @returns {unknown} The message hash.
   * @example
   * const hashMessage = await wallet.burnCurrency(burnCurrency);
   * await waitTillCompleted(client, hashMessage);
   */
  async burnCurrency(amount: bigint) {
    return await this.changeCurrencyAmount(amount, false);
  }

  private async changeCurrencyAmount(amount: bigint, mint: boolean) {
    let method = "burnCurrency";
    if (mint) {
      method = "mintCurrency";
    }

    const callData = encodeFunctionData({
      abi: Wallet.abi,
      functionName: method,
      args: [amount],
    });

    const { hash } = await this.requestToWallet({
      data: hexToBytes(callData),
      deploy: false,
    });

    return bytesToHex(hash);
  }

  /**
   * Send a raw signed message via the wallet.
   *
   * @async
   * @param {Uint8Array} rawMessage The message bytecode.
   * @returns {unknown} The message hash.
   */
  async sendRawInternalMessage(rawMessage: Uint8Array) {
    const { hash } = await this.requestToWallet({
      data: rawMessage,
      deploy: false,
    });

    return bytesToHex(hash);
  }

  /**
   * Deploys a new smart contract via the wallet.
   *
   * @async
   * @param {DeployParams} param0 The object representing the contract deployment params.
   * @param {DeployParams} param0.shardId The ID of the shard where the contract should be deployed.
   * @param {DeployParams} param0.bytecode The contract bytecode.
   * @param {DeployParams} param0.abi The contract ABI.
   * @param {DeployParams} param0.args The arbitrary arguments for deployment.
   * @param {DeployParams} param0.salt The arbitrary data for changing the contract address.
   * @param {DeployParams} param0.value The deployment message value.
   * @param {DeployParams} param0.feeCredit The deployment message fee credit.
   * @param {DeployParams} param0.seqno The deployment message seqno.
   * @param {DeployParams} param0.chainId The deployment message chain id.
   * @returns {unknown} The object containing the deployment message hash and the contract address.
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
      abi: Wallet.abi,
      functionName: "asyncDeploy",
      args: [shardId, value ?? 0n, hexData, salt],
    });

    const { hash } = await this.requestToWallet({
      data: hexToBytes(callData),
      deploy: false,
      seqno,
      chainId,
    });

    return {
      hash: bytesToHex(hash),
      address: bytesToHex(address),
    };
  }

  /**
   * Creates a new message and performs a synchronous call to the specified address.
   *
   * @async
   * @param {SendSyncMessageParams} param0 The object representing the message params.
   * @param {SendSyncMessageParams} param0.to The address where the message should be sent.
   * @param {SendSyncMessageParams} param0.data The message bytecode.
   * @param {SendSyncMessageParams} param0.abi The message abi.
   * @param {SendSyncMessageParams} param0.functionName The message function name for abi.
   * @param {SendSyncMessageParams} param0.args The message args for abi.
   * @param {SendMessageParams} param0.seqno The message sequence number.
   * @param {SendMessageParams} param0.gas The message gas.
   * @param {SendMessageParams} param0.value The message value.
   * @returns {unknown} The message hash.
   * @example
   * const anotherAddress = WalletV1.calculateWalletAddress({
   *     pubKey: pubkey,
   *     shardId: 1,
   *     salt: 200n,
   *   });
   * await wallet.sendMessage({
   *     to: anotherAddress,
   *     value: 10n,
   *     gas: 100000n,
   *   });
   */
  async syncSendMessage({
    to,
    data,
    abi,
    functionName,
    args,
    seqno,
    gas,
    value,
  }: SendSyncMessageParams) {
    const hexTo = refineAddress(to);
    const hexData = refineFunctionHexData({ data, abi, functionName, args });

    const callData = encodeFunctionData({
      abi: Wallet.abi,
      functionName: "syncCall",
      args: [hexTo, gas, value, hexData],
    });

    const { hash } = await this.requestToWallet({
      data: hexToBytes(callData),
      deploy: false,
      seqno,
    });

    return bytesToHex(hash);
  }

  /**
   * Returns the wallet balance.
   *
   * @async
   * @returns {unknown} The wallet balance.
   */
  async getBalance() {
    return this.client.getBalance(this.address, "latest");
  }
}
