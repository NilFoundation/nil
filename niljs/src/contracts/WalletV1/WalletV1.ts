import type { Abi } from "abitype";
import invariant from "tiny-invariant";
import { bytesToHex, encodeFunctionData } from "viem";
import type { PublicClient } from "../../clients/PublicClient.js";
import { prepareDeployPart } from "../../encoding/deployPart.js";
import { externalMessageEncode } from "../../encoding/externalMessage.js";
import { hexToBytes } from "../../encoding/fromHex.js";
import { toHex } from "../../encoding/toHex.js";
import type { ISigner } from "../../signers/index.js";
import type { IDeployData } from "../../types/IDeployData.js";
import { getShardIdFromAddress, refineAddress } from "../../utils/address.js";
import { refineCompressedPublicKey, refineSalt } from "../../utils/refiners.js";
import { code } from "./Wallet-bin.js";
import WalletAbi from "./Wallet.abi.json";
import type {
  DeployParams,
  RequestParams,
  SendMessageParams,
  SendSyncMessageParams,
  WalletV1Config,
} from "./types/index.js";

/**
 * WalletV1 is a class used for performing operations on the cluster that require authentication.
 *
 * @class WalletV1
 * @typedef {WalletV1}
 */
export class WalletV1 {
  /**
   * The wallet bytecode.
   *
   * @static
   * @type {*}
   */
  static code = hexToBytes(code);
  /**
   * The wallet ABI.
   *
   * @static
   * @type {Abi}
   */
  static abi = WalletAbi as Abi;

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

     const pubkey = await signer.getPublicKey();

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
      abi: WalletAbi as Abi,
      bytecode: WalletV1.code,
      args: [bytesToHex(pubKey)],
      salt: salt,
      shard: shardId,
    });
    return address;
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
   * @type {Uint8Array}
   */
  address: Uint8Array;

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
  constructor({
    pubkey,
    shardId,
    address,
    client,
    salt,
    signer,
  }: WalletV1Config) {
    this.pubkey = refineCompressedPublicKey(pubkey);
    this.client = client;
    this.signer = signer;
    invariant(
      !(salt && address),
      "You should use salt and shard for calculating address or address itself, not both to avoid issue.",
    );
    this.address = address
      ? refineAddress(address)
      : WalletV1.calculateWalletAddress({
          pubKey: this.pubkey,
          shardId,
          salt,
        });
    if (salt) {
      this.salt = refineSalt(salt);
    }
    this.shardId = getShardIdFromAddress(toHex(this.address));
  }

  /**
   * Converts the wallet address into a hexadecimal.
   *
   * @returns {String}
   */
  getAddressHex() {
    return bytesToHex(this.address);
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
         endpoint: "http://127.0.0.1:8529",
       }),
       shardId: 1,
     });
   * const signer = new LocalECDSAKeySigner({
       privateKey: generateRandomPrivateKey(),
     });
   * const faucet = new Faucet(client);
   * await faucet.withdrawTo(walletAddress, 100000n);
   * const pubkey = await signer.getPublicKey();
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
      await this.client.getBalance(this.getAddressHex(), "latest"),
      await this.client
        .getCode(this.getAddressHex(), "latest")
        .catch(() => Uint8Array.from([])),
    ]);

    invariant(code.length === 0, "Contract already deployed");
    invariant(balance > 0n, "Insufficient balance");

    const { data } = prepareDeployPart({
      abi: WalletAbi as Abi,
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
        const code = await this.client.getCode(this.getAddressHex(), "latest");
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
    const code = await this.client.getCode(this.getAddressHex(), "latest");
    return code.length > 0;
  }

  /**
   * Performs a request to the wallet.
   *
   * @async
   * @param {RequestParams} requestParams The object representing the request parameters.
   * @param {boolean} [send=true] The flag that determines whether the request is sent when the function is called.
   * @returns {Promise<{ raw: Uint8Array; hash: Uint8Array }>} The message bytecode and hash.
   */
  async requestToWallet(
    requestParams: RequestParams,
    send = true,
  ): Promise<{ raw: Uint8Array; hash: Uint8Array }> {
    const [seqno, chainId] = await Promise.all([
      requestParams.seqno ??
        this.client.getMessageCount(this.getAddressHex(), "latest"),
      requestParams.chainId ?? this.client.chainId(),
    ]);
    const encodedMessage = await externalMessageEncode(
      {
        isDeploy: requestParams.deploy,
        to: this.address,
        chainId: chainId,
        seqno,
        data: requestParams.data,
      },
      this.signer,
    );
    if (send) await this.client.sendRawMessage(encodedMessage.raw);
    return encodedMessage;
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
    deploy,
    seqno,
    feeCredit,
    value,
    tokens,
    chainId,
  }: SendMessageParams) {
    const hexTo = bytesToHex(refineAddress(to));
    const hexRefundTo = bytesToHex(refineAddress(refundTo ?? this.address));
    const hexBounceTo = bytesToHex(refineAddress(bounceTo ?? this.address));
    const hexData = data
      ? data instanceof Uint8Array
        ? bytesToHex(data)
        : data
      : "0x";

    const callData = encodeFunctionData({
      abi: WalletAbi,
      functionName: "asyncCall",
      args: [
        hexTo,
        hexRefundTo,
        hexBounceTo,
        feeCredit,
        !!deploy,
        tokens ?? [],
        value ?? 0n,
        hexData,
      ],
    });

    const { hash } = await this.requestToWallet({
      data: hexToBytes(callData),
      deploy: false,
      seqno,
      chainId,
    });

    return bytesToHex(hash);
  }

  async setCurrencyName(name: string) {

    const callData = encodeFunctionData({
      abi: WalletAbi,
      functionName: "setCurrencyName",
      args: [
        name
      ],
    });

    const { hash } = await this.requestToWallet({
      data: hexToBytes(callData),
      deploy: false,
    });

    return bytesToHex(hash);
  }

  async mintCurrency(amount: bigint) {

    const callData = encodeFunctionData({
      abi: WalletAbi,
      functionName: "mintCurrency",
      args: [
        amount
      ],
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
      invariant(
          !(abi || args),
        "ABI and args should be provided together or not provided at all.",
      );
      deployData = {
        shard: shardId,
        bytecode,
        salt,
      };
    }

    const { data, address } = prepareDeployPart(deployData);

    const hash = await this.sendMessage({
      to: address,
      refundTo: this.getAddressHex(),
      data,
      value: value ?? 0n,
      deploy: true,
      feeCredit,
      seqno,
      chainId,
    });

    return {
      hash,
      address: bytesToHex(address),
    };
  }

  /**
   * Send a message synchronously via the wallet.
   *
   * @async
   * @param {SendSyncMessageParams} param0 The object representing the message params.
   * @param {SendSyncMessageParams} param0.to The address where the message should be sent.
   * @param {SendSyncMessageParams} param0.data The message bytecode.
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
    seqno,
    gas,
    value,
  }: SendSyncMessageParams) {
    const hexTo = bytesToHex(refineAddress(to));
    const hexData = data
      ? data instanceof Uint8Array
        ? bytesToHex(data)
        : data
      : "0x";

    const callData = encodeFunctionData({
      abi: WalletAbi,
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
    return this.client.getBalance(this.getAddressHex(), "latest");
  }
}
