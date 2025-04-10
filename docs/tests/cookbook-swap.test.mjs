import { createRequire } from "node:module";
const require = createRequire(import.meta.url);

const { HttpTransport, PublicClient, generateSmartAccount } = require("@nilfoundation/niljs");
import { SWAP_MATCH_COMPILATION_COMMAND } from "./compilationCommands";

import { FAUCET_GLOBAL, RPC_GLOBAL } from "./globals";
const util = require("node:util");
const exec = util.promisify(require("node:child_process").exec);
const RPC_ENDPOINT = RPC_GLOBAL;
const FAUCET_ENDPOINT = FAUCET_GLOBAL;
import fs from "node:fs/promises";
import path from "node:path";

const __dirname = path.dirname(__filename);

let SWAP_MATCH_BYTECODE;
let SWAP_MATCH_ABI;

beforeAll(async () => {
  await exec(SWAP_MATCH_COMPILATION_COMMAND);
  const swapFile = await fs.readFile(path.resolve(__dirname, "./SwapMatch/SwapMatch.bin"), "utf8");
  const swapBytecode = `0x${swapFile}`;

  const swapAbiFile = await fs.readFile(
    path.resolve(__dirname, "./SwapMatch/SwapMatch.abi"),
    "utf8",
  );

  const swapAbi = JSON.parse(swapAbiFile);

  SWAP_MATCH_BYTECODE = swapBytecode;
  SWAP_MATCH_ABI = swapAbi;
});

describe.sequential("Nil.js handles the full swap tutorial flow", async () => {
  test.sequential(
    "the Cookbook tutorial flow passes for SwapMatch",
    async () => {
      //startTwoNewSmartAccountsDeploy
      const SALT = BigInt(Math.floor(Math.random() * 10000));

      const client = new PublicClient({
        transport: new HttpTransport({
          endpoint: RPC_ENDPOINT,
        }),
        shardId: 1,
      });

      const smartAccountOne = await generateSmartAccount({
        shardId: 2,
        rpcEndpoint: RPC_ENDPOINT,
        faucetEndpoint: FAUCET_ENDPOINT,
      });

      const smartAccountTwo = await generateSmartAccount({
        shardId: 3,
        rpcEndpoint: RPC_ENDPOINT,
        faucetEndpoint: FAUCET_ENDPOINT,
      });

      const smartAccountOneAddress = smartAccountOne.address;
      const smartAccountTwoAddress = smartAccountTwo.address;

      //endTwoNewSmartAccountsDeploy

      //startDeploymentOfSwapMatch
      const smartAccount = await generateSmartAccount({
        shardId: 1,
        rpcEndpoint: RPC_ENDPOINT,
        faucetEndpoint: FAUCET_ENDPOINT,
      });

      const { address: swapMatchAddress, tx: deploymentTransaction } =
        await smartAccount.deployContract({
          bytecode: SWAP_MATCH_BYTECODE,
          value: 0n,
          feeCredit: 100_000_000_000_000n,
          salt: SALT,
          shardId: 4,
        });

      const receipts = await deploymentTransaction.wait();
      //endDeploymentOfSwapMatch
      function bigIntReplacer(unusedKey, value) {
        return typeof value === "bigint" ? value.toString() : value;
      }
      console.log("Deployment receipts: ", JSON.stringify(receipts, bigIntReplacer));
      expect(receipts.some((receipt) => !receipt.success)).toBe(false);

      const code = await client.getCode(swapMatchAddress, "latest");

      expect(code).toBeDefined();
      expect(code.length).toBeGreaterThan(10);

      //startTokenCreation
      {
        const tx = await smartAccountOne.mintToken(100_000_000n);
        await tx.wait();
      }

      {
        const tx = await smartAccountTwo.mintToken(100_000_000n);
        await tx.wait();
      }
      //endTokenCreation

      //startFirstSendRequest
      {
        const gasPrice = await client.getGasPrice(smartAccountOne.shardId);
        const tx = await smartAccountOne.sendTransaction({
          to: swapMatchAddress,
          tokens: [
            {
              id: smartAccountOneAddress,
              amount: 30_000_000n,
            },
          ],
          abi: SWAP_MATCH_ABI,
          functionName: "placeSwapRequest",
          args: [20_000_000n, smartAccountTwoAddress],
          feeCredit: gasPrice * 1_000_000n,
        });

        await tx.wait();
      }
      //endFirstSendRequest

      //startSecondSendRequest
      {
        const gasPrice = await client.getGasPrice(smartAccountTwo.shardId);
        const tx = await smartAccountTwo.sendTransaction({
          to: swapMatchAddress,
          tokens: [
            {
              id: smartAccountTwoAddress,
              amount: 50_000_000n,
            },
          ],
          abi: SWAP_MATCH_ABI,
          functionName: "placeSwapRequest",
          args: [10_000_000n, smartAccountOneAddress],
          feeCredit: gasPrice * 1_000_000n,
        });

        await tx.wait();
      }

      //endSecondSendRequest

      //startFinalChecks
      const tokensOne = await client.getTokens(smartAccountOneAddress, "latest");
      const tokensTwo = await client.getTokens(smartAccountTwoAddress, "latest");
      console.log("Smart account 1 tokens: ", tokensOne);
      console.log("Smart account 2 tokens: ", tokensTwo);
      //endFinalChecks
    },
    70000,
  );
});
