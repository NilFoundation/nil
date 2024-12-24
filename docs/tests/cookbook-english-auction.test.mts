//startImportStatements
import {
  convertEthToWei,
  Faucet,
  generateRandomPrivateKey,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  waitTillCompleted,
  WalletV1,
} from "@nilfoundation/niljs";
import { encodeFunctionData, type Abi } from "viem";
//endImportStatements
import { AUCTION_COMPILATION_COMMAND, NFT_COMPILATION_COMMAND } from "./compilationCommands";
import { RPC_GLOBAL } from "./globals";
import fs from "node:fs/promises";
import path from "node:path";

const util = require("node:util");
const exec = util.promisify(require("node:child_process").exec);
const RPC_ENDPOINT = RPC_GLOBAL;

const __dirname = path.dirname(__filename);

let NFT_BYTECODE;
let NFT_ABI;
let AUCTION_BYTECODE;
let AUCTION_ABI;

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

      const faucet = new Faucet(client);

      const pkey = generateRandomPrivateKey();

      const signer = new LocalECDSAKeySigner({
        privateKey: pkey,
      });

      const pubkey = signer.getPublicKey();

      const wallet = new WalletV1({
        pubkey: pubkey,
        client: client,
        signer: signer,
        shardId: 1,
        salt: SALT,
      });

      const walletAddress = wallet.address;

      await faucet.withdrawToWithRetry(walletAddress, convertEthToWei(10));

      await wallet.selfDeploy(true);
      const gasPrice = await client.getGasPrice(1);

      const { address: addressNFT, hash: hashNFT } = await wallet.deployContract({
        salt: SALT,
        shardId: 1,
        bytecode: NFT_BYTECODE,
        abi: NFT_ABI,
        args: [],
        feeCredit: 3_000_000n * gasPrice,
      });

      const receiptsNFT = await waitTillCompleted(client, hashNFT);

      const { address: addressAuction, hash: hashAuction } = await wallet.deployContract({
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
      const startAuctionHash = await wallet.sendMessage({
        to: addressAuction,
        feeCredit: 1_000_000n * gasPrice,
        data: encodeFunctionData({
          abi: AUCTION_ABI,
          functionName: "start",
        }),
      });

      const receiptsStart = await waitTillCompleted(client, startAuctionHash);

      //endStartAuction

      expect(receiptsStart.some((receipt) => !receipt.success)).toBe(false);

      //startBid
      const signerTwo = new LocalECDSAKeySigner({
        privateKey: pkey,
      });

      const pubkeyTwo = signer.getPublicKey();

      const walletTwo = new WalletV1({
        pubkey: pubkeyTwo,
        client: client,
        signer: signerTwo,
        shardId: 2,
        salt: SALT,
      });

      const walletTwoAddress = walletTwo.address;

      await faucet.withdrawToWithRetry(walletTwoAddress, convertEthToWei(10));

      await walletTwo.selfDeploy(true);

      const bidHash = await walletTwo.sendMessage({
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

      const endHash = await wallet.sendMessage({
        to: addressAuction,
        feeCredit: 1_000_000n * gasPrice,
        data: encodeFunctionData({
          abi: AUCTION_ABI,
          functionName: "end",
        }),
      });

      const receiptsEnd = await waitTillCompleted(client, endHash);

      const result = await client.getCurrencies(walletTwoAddress, "latest");

      console.log(result);

      //endEndAuction

      expect(receiptsEnd.some((receipt) => !receipt.success)).toBe(false);

      expect(Object.keys(result)).toContain(addressNFT);
      expect(Object.values(result)).toContain(1n);
    },
    80000,
  );
});
