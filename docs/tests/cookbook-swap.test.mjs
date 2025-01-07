import { createRequire } from "node:module";
const require = createRequire(import.meta.url);

const {
  Faucet,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
  generateRandomPrivateKey,
  convertEthToWei,
  waitTillCompleted,
} = require("@nilfoundation/niljs");
import { SWAP_MATCH_COMPILATION_COMMAND } from "./compilationCommands";

import { RPC_GLOBAL } from "./globals";
const util = require("node:util");
const exec = util.promisify(require("node:child_process").exec);
const RPC_ENDPOINT = RPC_GLOBAL;
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
      //startTwoNewWalletsDeploy
      const SALT = BigInt(Math.floor(Math.random() * 10000));

      const client = new PublicClient({
        transport: new HttpTransport({
          endpoint: RPC_ENDPOINT,
        }),
        shardId: 1,
      });

      const faucet = new Faucet(client);

      const signer = new LocalECDSAKeySigner({
        privateKey: generateRandomPrivateKey(),
      });

      const pubkey = signer.getPublicKey();

      const walletOne = new WalletV1({
        pubkey: pubkey,
        client,
        signer,
        shardId: 2,
        salt: SALT,
      });

      const walletTwo = new WalletV1({
        pubkey: pubkey,
        client,
        signer,
        shardId: 3,
        salt: SALT,
      });

      const walletOneAddress = walletOne.address;
      const walletTwoAddress = walletTwo.address;

      const fundingWalletOne = await faucet.withdrawToWithRetry(
        walletOneAddress,
        convertEthToWei(10),
      );

      const fundingWalletTwo = await faucet.withdrawToWithRetry(
        walletTwoAddress,
        convertEthToWei(10),
      );

      await walletOne.selfDeploy(true);
      await walletTwo.selfDeploy(true);

      //endTwoNewWalletsDeploy
      expect(walletOneAddress).toBeDefined();
      const walletOneCode = await client.getCode(walletOneAddress, "latest");
      expect(walletOneCode).toBeDefined();
      expect(walletOneCode.length).toBeGreaterThan(10);

      expect(walletTwoAddress).toBeDefined();
      const walletTwoCode = await client.getCode(walletTwoAddress, "latest");
      expect(walletTwoCode).toBeDefined();
      expect(walletTwoCode.length).toBeGreaterThan(10);

      //startDeploymentOfSwapMatch
      const wallet = new WalletV1({
        pubkey: pubkey,
        client,
        signer,
        shardId: 1,
        salt: SALT,
      });

      const fundingNewWallet = await faucet.withdrawToWithRetry(
        wallet.address,
        convertEthToWei(10),
      );

      await wallet.selfDeploy(true);

      const { address: swapMatchAddress, hash: deploymentMessageHash } =
        await wallet.deployContract({
          bytecode: SWAP_MATCH_BYTECODE,
          value: 0n,
          feeCredit: 100_000_000n,
          salt: SALT,
          shardId: 4,
        });

      const receipts = await waitTillCompleted(client, deploymentMessageHash);
      //endDeploymentOfSwapMatch

      expect(receipts.some((receipt) => !receipt.success)).toBe(false);

      const code = await client.getCode(swapMatchAddress, "latest");

      expect(code).toBeDefined();
      expect(code.length).toBeGreaterThan(10);

      //startCurrencyCreation
      {
        const hashMessage = await walletOne.mintCurrency(100_000_000n);
        await waitTillCompleted(client, hashMessage);
      }

      {
        const hashMessage = await walletTwo.mintCurrency(100_000_000n);
        await waitTillCompleted(client, hashMessage);
      }
      //endCurrencyCreation

      //startFirstSendRequest
      {
        const gasPrice = await client.getGasPrice(2);
        const hashMessage = await walletOne.sendMessage({
          to: swapMatchAddress,
          tokens: [
            {
              id: walletOneAddress,
              amount: 30_000_000n,
            },
          ],
          abi: SWAP_MATCH_ABI,
          functionName: "placeSwapRequest",
          args: [20_000_000n, walletTwoAddress],
          feeCredit: gasPrice * 1_000_000_000n,
        });

        await waitTillCompleted(client, hashMessage);
      }
      //endFirstSendRequest

      //startSecondSendRequest
      {
        const gasPrice = await client.getGasPrice(3);
        const hashMessage = await walletTwo.sendMessage({
          to: swapMatchAddress,
          tokens: [
            {
              id: walletTwoAddress,
              amount: 50_000_000n,
            },
          ],
          abi: SWAP_MATCH_ABI,
          functionName: "placeSwapRequest",
          args: [10_000_000n, walletOneAddress],
          feeCredit: gasPrice * 1_000_000_000n,
        });

        await waitTillCompleted(client, hashMessage);
      }

      //endSecondSendRequest

      //startFinalChecks
      const tokensOne = await client.getCurrencies(walletOneAddress, "latest");
      const tokensTwo = await client.getCurrencies(walletTwoAddress, "latest");
      console.log("Wallet 1 tokens: ", tokensOne);
      console.log("Wallet 2 tokens: ", tokensTwo);
      //endFinalChecks
    },
    70000,
  );
});
