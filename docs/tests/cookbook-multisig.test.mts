//startImportStatements
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);

import {
  ExternalMessageEnvelope,
  Faucet,
  type Hex,
  HttpTransport,
  type ISigner,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
  bytesToHex,
  convertEthToWei,
  externalDeploymentMessage,
  generateRandomPrivateKey,
  hexToBytes,
  isHexString,
  refineAddress,
  waitTillCompleted,
} from "@nilfoundation/niljs";
import { secp256k1 } from "@noble/curves/secp256k1";

import { concatBytes, numberToBytesBE } from "@noble/curves/abstract/utils";
import { type Abi, encodeFunctionData } from "viem";

//endImportStatements

import { MULTISIG_COMPILATION_COMMAND } from "./compilationCommands";
import { RPC_GLOBAL } from "./globals";

const util = require("node:util");
const exec = util.promisify(require("node:child_process").exec);

import fs from "node:fs/promises";
import path from "node:path";
const __dirname = path.dirname(__filename);

let MULTISIG_WALLET_ABI: Abi;
let MULTISIG_WALLET_BYTECODE: `0x${string}`;

const RPC_ENDPOINT = RPC_GLOBAL;

beforeAll(async () => {
  await exec(MULTISIG_COMPILATION_COMMAND);
  const multisigFile = await fs.readFile(
    path.resolve(__dirname, "./MultiSigWallet/MultiSigWallet.bin"),
    "utf8",
  );
  const multisigBytecode = `0x${multisigFile}` as `0x${string}`;

  const multisigAbiFile = await fs.readFile(
    path.resolve(__dirname, "./MultiSigWallet/MultiSigWallet.abi"),
    "utf8",
  );
  const multisigAbi = JSON.parse(multisigAbiFile) as unknown as Abi;

  MULTISIG_WALLET_BYTECODE = multisigBytecode;
  MULTISIG_WALLET_ABI = multisigAbi;
});

//startRefineFunctionHexData
const refineFunctionHexData = ({
  data,
  abi,
  functionName,
  args,
}: {
  data?: Uint8Array | Hex;
  abi?: Abi;
  functionName?: string;
  args?: unknown[];
}): Hex => {
  if (!data && !abi) {
    return "0x";
  }
  if (data) {
    return typeof data === "string" ? data : bytesToHex(data);
  }
  if (!functionName) {
    throw new Error("Function name is required");
  }
  if (!abi) {
    throw new Error("ABI is required");
  }
  return encodeFunctionData({
    abi,
    functionName: functionName,
    args: args || [],
  });
};
//endRefineFunctionHexData

describe.sequential("the multisig wallet performs all operations internally", async () => {
  test.sequential(
    "signers can withdraw default tokens from the wallet internally",
    async () => {
      //startHelpers
      /**
       * MultisigSigner is a special signer that can create an array of signatures
       * when given a the data to sign.
       *
       * @class MultisigSigner
       * @typedef {MultisigSigner}
       * @implements {ISigner}
       */
      class MultisigSigner implements ISigner {
        private keys: Uint8Array[];
        constructor(keys: Uint8Array[]) {
          for (let i = 0; i < keys.length; i++) {
            if (keys[i].length !== 32) {
              throw new Error("Invalid key length");
            }
          }
          this.keys = keys;
        }

        async sign(data: Uint8Array): Promise<Uint8Array> {
          const fullSignatures = new Uint8Array(this.keys.length * 65);
          for (let i = 0; i < this.keys.length; i++) {
            const signature = secp256k1.sign(data, this.keys[i]);
            const { r, s, recovery } = signature;
            fullSignatures.set(
              concatBytes(
                numberToBytesBE(r, 32),
                numberToBytesBE(s, 32),
                numberToBytesBE(recovery, 1),
              ),
              i * 65,
            );
          }
          return fullSignatures;
        }
        getPublicKey(params: unknown): Promise<Uint8Array> {
          throw new Error("Method not implemented.");
        }
        getAddress(params: unknown): Promise<Uint8Array> {
          throw new Error("Method not implemented.");
        }
      }

      /**
       * MultiSigWallet is a 'helper' class for sending external messages
       * to the multi-signature wallet.
       *
       * @class MultiSigWallet
       * @typedef {MultiSigWallet}
       */
      class MultiSigWallet {
        private keys: Uint8Array[];
        private salt: bigint;
        private chainId: number;
        private client: PublicClient;
        public address: Hex;
        constructor(
          keys: (Uint8Array | Hex)[],
          salt: bigint,
          chainId: number,
          shardId: number,
          client: PublicClient,
        ) {
          this.keys = keys.map((key) => {
            if (isHexString(key)) {
              return hexToBytes(key);
            }
            return key;
          });
          this.salt = salt;
          this.address = MultiSigWallet.calculateAddress(chainId, shardId, keys, salt);
          this.chainId = chainId;
          this.client = client;
        }
        static calculateAddress(
          chainId: number,
          shardId: number,
          keys: (Uint8Array | Hex)[],
          salt: bigint,
        ) {
          const msg = externalDeploymentMessage(
            {
              abi: MULTISIG_WALLET_ABI,
              args: [keys],
              bytecode: MULTISIG_WALLET_BYTECODE,
              salt,
              shard: shardId,
            },
            chainId,
          );
          return msg.hexAddress();
        }

        async sendTransaction({
          to,
          refundTo,
          bounceTo,
          data,
          abi,
          functionName,
          args,
          deploy,
          seqno,
          feeCredit,
          value,
          tokens,
          chainId,
        }: SendMessageParams) {
          const refinedSeqno = seqno ?? (await this.client.getMessageCount(this.address, "latest"));

          const hexTo = bytesToHex(refineAddress(to));
          const hexRefundTo = bytesToHex(refineAddress(refundTo ?? this.address));
          const hexBounceTo = bytesToHex(refineAddress(bounceTo ?? this.address));
          const hexData = refineFunctionHexData({ data, abi, functionName, args });

          const callData = encodeFunctionData({
            abi: MULTISIG_WALLET_ABI,
            functionName: "asyncCall",
            args: [hexTo, hexRefundTo, hexBounceTo, feeCredit, tokens ?? [], value ?? 0n, hexData],
          });
          const msg = new ExternalMessageEnvelope({
            isDeploy: !!deploy,
            data: hexToBytes(callData),
            to: hexToBytes(this.address),
            seqno: refinedSeqno,
            chainId: chainId ?? this.chainId,
            authData: new Uint8Array(0),
          });

          const { raw } = await msg.encodeWithSignature(signer);
          const hash = await this.client.sendRawMessage(raw);
          return hash;
        }
      }

      //endHelpers

      //startInitialUsageFlow
      const SALT = BigInt(Math.floor(Math.random() * 10000));

      const client = new PublicClient({
        transport: new HttpTransport({
          endpoint: RPC_ENDPOINT,
        }),
        shardId: 1,
      });

      const faucet = new Faucet(client);

      const pkOne = generateRandomPrivateKey();
      const pkTwo = generateRandomPrivateKey();
      const pkThree = generateRandomPrivateKey();

      const signerOne = new LocalECDSAKeySigner({
        privateKey: pkOne,
      });
      const signerTwo = new LocalECDSAKeySigner({
        privateKey: pkTwo,
      });
      const signerThree = new LocalECDSAKeySigner({
        privateKey: pkThree,
      });

      const pubkeyOne = await signerOne.getPublicKey();
      const pubkeyTwo = await signerTwo.getPublicKey();
      const pubkeyThree = await signerThree.getPublicKey();

      const walletOne = new WalletV1({
        pubkey: pubkeyOne,
        client: client,
        signer: signerOne,
        shardId: 1,
        salt: SALT,
      });

      const walletOneAddress = walletOne.address;

      const walletTwo = new WalletV1({
        pubkey: pubkeyTwo,
        client: client,
        signer: signerTwo,
        shardId: 1,
        salt: SALT,
      });

      await faucet.withdrawToWithRetry(walletOneAddress, convertEthToWei(10));

      await walletOne.selfDeploy(true);

      //endInitialUsageFlow

      expect(walletOneAddress).toBeDefined();
      const walletOneCode = await client.getCode(walletOneAddress, "latest");
      expect(walletOneCode).toBeDefined();
      expect(walletOneCode.length).toBeGreaterThan(10);
      const gasPrice = await client.getGasPrice(1);

      //startMultiSigDeployment
      const hexKeys = [pubkeyOne, pubkeyTwo, pubkeyThree].map((key) => bytesToHex(key));

      const { address: multiSigWalletAddress, hash: deploymentMessageHash } =
        await walletOne.deployContract({
          bytecode: MULTISIG_WALLET_BYTECODE,
          abi: MULTISIG_WALLET_ABI,
          args: [hexKeys],
          value: convertEthToWei(0.001),
          feeCredit: 10_000_000n * gasPrice,
          salt: SALT,
          shardId: 1,
        });

      const signer = new MultisigSigner([pkOne, pkTwo, pkThree].map((x) => hexToBytes(x)));

      const receipts = await waitTillCompleted(client, deploymentMessageHash);

      //endMultiSigDeployment

      expect(receipts.some((receipt) => !receipt.success)).toBe(false);

      const code = await client.getCode(multiSigWalletAddress, "latest");

      expect(code).toBeDefined();
      expect(code.length).toBeGreaterThan(10);

      //startTransfer
      const chainId = await client.chainId();

      const multiWallet = new MultiSigWallet(hexKeys, SALT, chainId, 1, client);

      const withdrawalHash = await multiWallet.sendTransaction({
        to: walletTwo.address,
        value: convertEthToWei(0.000001),
        feeCredit: 10_000_000n * gasPrice,
      });

      await waitTillCompleted(client, withdrawalHash);

      const balance = await client.getBalance(bytesToHex(walletTwo.address), "latest");

      //endTransfer

      expect(balance).toBe(convertEthToWei(0.000001));
    },
    100000,
  );
});
