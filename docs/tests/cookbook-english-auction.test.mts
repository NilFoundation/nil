import fs from "node:fs/promises";
import path from "node:path";
import util from "node:util";

import { AUCTION_COMPILATION_COMMAND, NFT_COMPILATION_COMMAND } from "./compilationCommands";
import { FAUCET_GLOBAL, RPC_GLOBAL } from "./globals";

//startImportStatements
import {
  HttpTransport,
  PublicClient,
  generateSmartAccount,
  waitTillCompleted,
} from "@nilfoundation/niljs";
import { type Abi, encodeFunctionData } from "viem";
//endImportStatements

const exec = util.promisify(require("node:child_process").exec);
const RPC_ENDPOINT = RPC_GLOBAL;
const FAUCET_ENDPOINT = FAUCET_GLOBAL;

const __dirname = path.dirname(__filename);

let NFT_BYTECODE: `0x${string}`;
let NFT_ABI: Abi;
let AUCTION_BYTECODE: `0x${string}`;
let AUCTION_ABI: Abi;

beforeAll(async () => {
  await exec(NFT_COMPILATION_COMMAND);
  await exec(AUCTION_COMPILATION_COMMAND);

  const nftFile = await fs.readFile(path.resolve(__dirname, "./NFT/NFT.bin"), "utf8");
  const nftBytecode = `0x${nftFile}` as `0x${string}`;

  const nftAbiFile = await fs.readFile(path.resolve(__dirname, "./NFT/NFT.abi"), "utf8");

  const nftAbi = JSON.parse(nftAbiFile) as unknown as Abi;

  const auctionFile = await fs.readFile(
    path.resolve(__dirname, "./EnglishAuction/EnglishAuction.bin"),
    "utf8",
  );
  const auctionBytecode = `0x${auctionFile}` as `0x${string}`;

  const auctionAbiFile = await fs.readFile(
    path.resolve(__dirname, "./EnglishAuction/EnglishAuction.abi"),
    "utf8",
  );

  const auctionAbi = JSON.parse(auctionAbiFile) as unknown as Abi;

  NFT_ABI = nftAbi;
  NFT_BYTECODE = nftBytecode;
  AUCTION_ABI = auctionAbi;
  AUCTION_BYTECODE = auctionBytecode;
});

describe.sequential("Nil.js can fully interact with EnglishAuction", async () => {
  test.sequential(
    "Nil.js can start, bid, and end the auction",
    async () => {
      //startInitialDeployments
      const SALT = BigInt(Math.floor(Math.random() * 10000));

      const client = new PublicClient({
        transport: new HttpTransport({
          endpoint: RPC_ENDPOINT,
        }),
        shardId: 1,
      });

      const smartAccount = await generateSmartAccount({
        shardId: 1,
        rpcEndpoint: RPC_ENDPOINT,
        faucetEndpoint: FAUCET_ENDPOINT,
      });

      const gasPrice = await client.getGasPrice();

      const { address: addressNFT, hash: hashNFT } = await smartAccount.deployContract({
        salt: SALT,
        shardId: 1,
        bytecode: NFT_BYTECODE,
        abi: NFT_ABI,
        args: [],
        feeCredit: 3_000_000n * gasPrice,
      });

      const receiptsNFT = await waitTillCompleted(client, hashNFT);

      const { address: addressAuction, hash: hashAuction } = await smartAccount.deployContract({
        salt: SALT,
        shardId: 3,
        bytecode: AUCTION_BYTECODE,
        value: 50_000n,
        abi: AUCTION_ABI,
        args: [addressNFT],
        feeCredit: 5_000_000n * gasPrice,
      });

      const receiptsAuction = await waitTillCompleted(client, hashAuction);

      //endInitialDeployments

      expect(receiptsNFT.some((receipt) => !receipt.success)).toBe(false);
      expect(receiptsAuction.some((receipt) => !receipt.success)).toBe(false);

      const codeNFT = await client.getCode(addressNFT, "latest");
      const codeAuction = await client.getCode(addressAuction, "latest");

      expect(codeNFT).toBeDefined;
      expect(codeAuction).toBeDefined;
      expect(codeNFT.length).toBeGreaterThan(10);
      expect(codeAuction.length).toBeGreaterThan(10);

      //startStartAuction
      const changeOwnershipHash = await smartAccount.sendTransaction({
        to: addressNFT,
        feeCredit: 500_000n * gasPrice,
        data: encodeFunctionData({
          abi: NFT_ABI,
          functionName: "changeOwnershipToAuction",
          args: [addressAuction],
        }),
      });

      const receiptsOwnership = await waitTillCompleted(client, changeOwnershipHash);

      const startAuctionHash = await smartAccount.sendTransaction({
        to: addressAuction,
        feeCredit: 1_000_000n * gasPrice,
        data: encodeFunctionData({
          abi: AUCTION_ABI,
          functionName: "start",
          args: [],
        }),
      });

      const receiptsStart = await waitTillCompleted(client, startAuctionHash);

      //endStartAuction
      expect(receiptsOwnership.some((receipt) => !receipt.success)).toBe(false);
      expect(receiptsStart.some((receipt) => !receipt.success)).toBe(false);

      //startBid
      const smartAccountTwo = await generateSmartAccount({
        shardId: 2,
        rpcEndpoint: RPC_ENDPOINT,
        faucetEndpoint: FAUCET_ENDPOINT,
      });

      const bidHash = await smartAccountTwo.sendTransaction({
        to: addressAuction,
        feeCredit: 1_000_000n * gasPrice,
        data: encodeFunctionData({
          abi: AUCTION_ABI,
          functionName: "bid",
          args: [],
        }),
        value: 300_000n,
      });

      const receiptsBid = await waitTillCompleted(client, bidHash);

      //endBid
      expect(receiptsBid.some((receipt) => !receipt.success)).toBe(false);
      //startEndAuction

      const endHash = await smartAccount.sendTransaction({
        to: addressAuction,
        feeCredit: 1_000_000n * gasPrice,
        data: encodeFunctionData({
          abi: AUCTION_ABI,
          functionName: "end",
          args: [],
        }),
      });

      const receiptsEnd = await waitTillCompleted(client, endHash);

      const result = await client.getTokens(smartAccountTwo.address, "latest");

      console.log(result);

      //endEndAuction

      expect(receiptsEnd.some((receipt) => !receipt.success)).toBe(false);

      expect(Object.keys(result)).toContain(addressNFT);
      expect(Object.values(result)).toContain(1n);
    },
    80000,
  );
});
