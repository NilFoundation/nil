//startImportStatements
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);

import {
  ExternalTransactionEnvelope,
  type Hex,
  HttpTransport,
  type ISigner,
  PublicClient,
  type SendTransactionParams,
  bytesToHex,
  calculateAddress,
  convertEthToWei,
  externalDeploymentTransaction,
  generateRandomPrivateKey,
  generateSmartAccount,
  getPublicKey,
  hexToBytes,
  isHexString,
  refineAddress,
  refineSalt,
  waitTillCompleted,
} from "@nilfoundation/niljs";
import { secp256k1 } from "@noble/curves/secp256k1";

import { concatBytes, numberToBytesBE } from "@noble/curves/abstract/utils";
import { type Abi, encodeFunctionData } from "viem";

//endImportStatements

import { MULTISIG_COMPILATION_COMMAND } from "./compilationCommands";
import { FAUCET_GLOBAL, RPC_GLOBAL } from "./globals";

const util = require("node:util");
const exec = util.promisify(require("node:child_process").exec);

import fs from "node:fs/promises";
import path from "node:path";
const __dirname = path.dirname(__filename);

let MULTISIG_SMART_ACCOUNT_ABI: Abi;
let MULTISIG_SMART_ACCOUNT_BYTECODE: `0x${string}`;

const RPC_ENDPOINT = RPC_GLOBAL;
const FAUCET_ENDPOINT = FAUCET_GLOBAL;

beforeAll(async () => {
  await exec(MULTISIG_COMPILATION_COMMAND);
  const multisigFile = await fs.readFile(
    path.resolve(__dirname, "./MultiSigSmartAccount/MultiSigSmartAccount.bin"),
    "utf8",
  );
  const multisigBytecode = `0x${multisigFile}` as `0x${string}`;

  const multisigAbiFile = await fs.readFile(
    path.resolve(__dirname, "./MultiSigSmartAccount/MultiSigSmartAccount.abi"),
    "utf8",
  );
  const multisigAbi = JSON.parse(multisigAbiFile) as unknown as Abi;

  MULTISIG_SMART_ACCOUNT_BYTECODE = multisigBytecode;
  MULTISIG_SMART_ACCOUNT_ABI = multisigAbi;
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

describe.sequential("the multisig smart account performs all operations internally", async () => {
  test.sequential(
    "signers can withdraw default tokens from the smart account internally",
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
        getPublicKey(): Uint8Array {
          throw new Error("Method not implemented.");
        }
        getAddress(params: unknown): Uint8Array {
          throw new Error("Method not implemented.");
        }
      }

      /**
       * MultiSigSmartAccount is a 'helper' class for sending external transactions
       * to the multi-signature smart account.
       *
       * @class MultiSigSmartAccount
       * @typedef {MultiSigSmartAccount}
       */
      class MultiSigSmartAccount {
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
          this.address = MultiSigSmartAccount.calculateAddress(chainId, shardId, keys, salt);
          this.chainId = chainId;
          this.client = client;
        }
        static calculateAddress(
          chainId: number,
          shardId: number,
          keys: (Uint8Array | Hex)[],
          salt: bigint,
        ) {
          const txn = externalDeploymentTransaction(
            {
              abi: MULTISIG_SMART_ACCOUNT_ABI,
              args: [keys],
              bytecode: MULTISIG_SMART_ACCOUNT_BYTECODE,
              salt,
              shard: shardId,
            },
            chainId,
          );
          return txn.hexAddress();
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
        }: SendTransactionParams) {
          const refinedSeqno =
            seqno ?? (await this.client.getTransactionCount(this.address, "latest"));

          const hexTo = refineAddress(to);
          const hexRefundTo = refineAddress(refundTo ?? this.address);
          const hexBounceTo = refineAddress(bounceTo ?? this.address);
          const hexData = refineFunctionHexData({ data, abi, functionName, args });

          const callData = encodeFunctionData({
            abi: MULTISIG_SMART_ACCOUNT_ABI,
            functionName: "asyncCall",
            args: [hexTo, hexRefundTo, hexBounceTo, feeCredit, tokens ?? [], value ?? 0n, hexData],
          });
          const txn = new ExternalTransactionEnvelope({
            isDeploy: !!deploy,
            data: hexToBytes(callData),
            to: hexToBytes(this.address),
            seqno: refinedSeqno,
            chainId: chainId ?? this.chainId,
            authData: new Uint8Array(0),
          });

          const { raw } = await txn.encodeWithSignature(signer);
          const hash = await this.client.sendRawTransaction(raw);
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

      const pkOne = generateRandomPrivateKey();
      const pkTwo = generateRandomPrivateKey();
      const pkThree = generateRandomPrivateKey();

      const smartAccount = await generateSmartAccount({
        shardId: 1,
        rpcEndpoint: RPC_ENDPOINT,
        faucetEndpoint: FAUCET_ENDPOINT,
      });

      const gasPrice = await client.getGasPrice(1);

      // A random address
      const dstAddress = refineAddress(calculateAddress(1, Uint8Array.of(1), refineSalt(SALT)));

      //endInitialUsageFlow

      //startMultiSigDeployment
      const hexKeys = [pkOne, pkTwo, pkThree].map((key) => getPublicKey(key));

      const { address: multiSigSmartAccountAddress, tx: deploymentTransactionTx } =
        await smartAccount.deployContract({
          bytecode: MULTISIG_SMART_ACCOUNT_BYTECODE,
          abi: MULTISIG_SMART_ACCOUNT_ABI,
          args: [hexKeys],
          value: convertEthToWei(0.001),
          feeCredit: 1_000_000n * gasPrice,
          salt: SALT,
          shardId: 1,
        });

      const signer = new MultisigSigner([pkOne, pkTwo, pkThree].map((x) => hexToBytes(x)));

      const receipts = await deploymentTransactionTx.wait();

      //endMultiSigDeployment

      expect(receipts.some((receipt) => !receipt.success)).toBe(false);

      const code = await client.getCode(multiSigSmartAccountAddress, "latest");

      expect(code).toBeDefined();
      expect(code.length).toBeGreaterThan(10);

      //startTransfer
      const chainId = await client.chainId();

      const multiSmartAccount = new MultiSigSmartAccount(hexKeys, SALT, chainId, 1, client);

      const withdrawalHash = await multiSmartAccount.sendTransaction({
        to: dstAddress,
        value: convertEthToWei(0.000001),
        feeCredit: 1_000_000n * gasPrice,
      });

      await waitTillCompleted(client, withdrawalHash);

      const balance = await client.getBalance(dstAddress, "latest");

      //endTransfer

      expect(balance).toBe(convertEthToWei(0.000001));
    },
    100000,
  );
});
