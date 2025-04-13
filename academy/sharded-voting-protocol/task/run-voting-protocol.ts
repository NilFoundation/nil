// tasks/e2e.ts

import {
  FaucetClient,
  HttpTransport,
  PublicClient,
  convertEthToWei,
  generateSmartAccount,
  getContract,
  type Block,
  waitTillCompleted,
} from "@nilfoundation/niljs";

import type { Abi } from "viem";
import * as dotenv from "dotenv";
import { task } from "hardhat/config";
dotenv.config();

// Extend Block to include dbTimestamp from =nil;
type NilBlock<T extends boolean = false> = Block<T> & {
  dbTimestamp: number;
};

// Waits until the current dbTimestamp is greater than or equal to the target
const waitUntilDbTimestamp = async (
  client: PublicClient,
  target: number,
  intervalMs = 2000
) => {
  while (true) {
    const block = (await client.getBlockByNumber(
      "latest",
      false,
      1
    )) as NilBlock;

    console.log(
      `⏳ Waiting... Current: ${block.dbTimestamp}, Target: ${target}`
    );

    if (block.dbTimestamp >= target) break;
    await new Promise((res) => setTimeout(res, intervalMs));
  }
};

const sleep = async (intervalMs: number) => {
  await new Promise((res) => setTimeout(res, intervalMs));
};

// Generates voting start and end time based on current block timestamp
function getVotingTimestamps(
  blockTime: number,
  offsetInSeconds = 60,
  durationInSeconds = 240
) {
  const roundedBlockTime = Math.ceil(blockTime / 10) * 10;
  const startTime = roundedBlockTime + offsetInSeconds;
  const endTime = startTime + durationInSeconds;

  return { startTime, endTime };
}

task("run-voting-protocol", "🔁 End-to-end test for sharded voting").setAction(
  async () => {
    try {
      console.log("🚀 Starting Sharded Voting E2E Test");

      const VoteManager = require("../artifacts/contracts/VoteManager.sol/VoteManager.json");
      const VoteShard = require("../artifacts/contracts/VoteShard.sol/VoteShard.json");

      const noOfShards = 4;
      const noOfChoices = 3;
      const candidate1 = 1;
      const candidate2 = 2;
      const candidate3 = 3;

      console.log("🔧 Initializing PublicClient and Faucet...");

      const client = new PublicClient({
        transport: new HttpTransport({
          endpoint: process.env.NIL_RPC_ENDPOINT as string,
        }),
      });

      const faucet = new FaucetClient({
        transport: new HttpTransport({
          endpoint: process.env.NIL_RPC_ENDPOINT as string,
        }),
      });

      console.log("👤 Generating Deployer and Voter Accounts...");

      const deployerWallet = await generateSmartAccount({
        shardId: 1,
        rpcEndpoint: process.env.NIL_RPC_ENDPOINT as string,
        faucetEndpoint: process.env.NIL_RPC_ENDPOINT as string,
      });

      const voter1 = await generateSmartAccount({
        shardId: 1,
        rpcEndpoint: process.env.NIL_RPC_ENDPOINT as string,
        faucetEndpoint: process.env.NIL_RPC_ENDPOINT as string,
      });
      const voter2 = await generateSmartAccount({
        shardId: 2,
        rpcEndpoint: process.env.NIL_RPC_ENDPOINT as string,
        faucetEndpoint: process.env.NIL_RPC_ENDPOINT as string,
      });
      const voter3 = await generateSmartAccount({
        shardId: 3,
        rpcEndpoint: process.env.NIL_RPC_ENDPOINT as string,
        faucetEndpoint: process.env.NIL_RPC_ENDPOINT as string,
      });
      const voter4 = await generateSmartAccount({
        shardId: 4,
        rpcEndpoint: process.env.NIL_RPC_ENDPOINT as string,
        faucetEndpoint: process.env.NIL_RPC_ENDPOINT as string,
      });

      console.log("💰 Funding accounts with NIL...");
      for (const acc of [deployerWallet, voter1, voter2, voter3, voter4]) {
        await faucet.topUpAndWaitUntilCompletion(
          {
            smartAccountAddress: acc.address,
            faucetAddress: process.env.NIL as `0x${string}`,
            amount: convertEthToWei(1000),
          },
          client
        );
      }

      const block = (await client.getBlockByNumber(
        "latest",
        false,
        1
      )) as NilBlock;
      const { startTime, endTime } = getVotingTimestamps(block.dbTimestamp);

      console.log("🏗 Deploying VoteManager on Shard 1...");
      const { address: voteManagerAddress, hash: deployVoteProtocolHash } =
        await deployerWallet.deployContract({
          shardId: 1,
          args: [noOfShards, noOfChoices, startTime, endTime],
          bytecode: VoteManager.bytecode as `0x${string}`,
          abi: VoteManager.abi as Abi,
          salt: BigInt(Math.floor(Math.random() * 10000)),
          value: 5000n,
          feeCredit: convertEthToWei(0.0001),
        });

      await waitTillCompleted(client, deployVoteProtocolHash);
      let txReceipt = await client.getTransactionReceiptByHash(
        deployVoteProtocolHash
      );
      if (txReceipt && !txReceipt.success) {
        throw new Error(
          `❌ VoteManager Deployment Failed.\nReason: ${txReceipt.errorMessage}\nStatus: ${txReceipt.status}`
        );
      }

      console.log(`✅ VoteManager deployed at ${voteManagerAddress}`);
      const voteManagerContract = getContract({
        client,
        abi: VoteManager.abi,
        address: voteManagerAddress,
      });

      console.log("📦 Deploying VoteShards...");
      const deployShardsResponse = await deployerWallet.sendTransaction({
        to: voteManagerAddress,
        functionName: "deployVotingShards",
        abi: VoteManager.abi as Abi,
        feeCredit: convertEthToWei(0.001),
      });

      await waitTillCompleted(client, deployShardsResponse);
      txReceipt = await client.getTransactionReceiptByHash(
        deployShardsResponse
      );
      if (txReceipt && !txReceipt.success) {
        throw new Error(
          `❌ Shard deployment failed.\nReason: ${txReceipt.errorMessage}\nStatus: ${txReceipt.status}`
        );
      }

      console.log("✅ All VoteShards deployed!");

      const voteShard1Address = (await voteManagerContract.read.getShardAddress(
        [1]
      )) as `0x${string}`;
      const voteShard2Address = (await voteManagerContract.read.getShardAddress(
        [2]
      )) as `0x${string}`;
      const voteShard3Address = (await voteManagerContract.read.getShardAddress(
        [3]
      )) as `0x${string}`;
      const voteShard4Address = (await voteManagerContract.read.getShardAddress(
        [4]
      )) as `0x${string}`;

      const voteShard1Contract = getContract({
        client,
        abi: VoteShard.abi,
        address: voteShard1Address,
      });
      const voteShard2Contract = getContract({
        client,
        abi: VoteShard.abi,
        address: voteShard2Address,
      });
      const voteShard3Contract = getContract({
        client,
        abi: VoteShard.abi,
        address: voteShard3Address,
      });
      const voteShard4Contract = getContract({
        client,
        abi: VoteShard.abi,
        address: voteShard4Address,
      });

      console.log("Waiting for voting to start...");
      await waitUntilDbTimestamp(client, startTime);

      console.log("🗳 Casting Votes...");
      const vote1 = await voter1.sendTransaction({
        to: voteShard1Address,
        functionName: "vote",
        args: [candidate1],
        abi: VoteShard.abi,
        feeCredit: convertEthToWei(0.001),
      });
      await waitTillCompleted(client, vote1);
      console.log("🗳️ Voter 1 voted for Candidate 1 in Shard 1✅");

      const vote2 = await voter2.sendTransaction({
        to: voteShard2Address,
        functionName: "vote",
        args: [candidate2],
        abi: VoteShard.abi,
        feeCredit: convertEthToWei(0.001),
      });
      await waitTillCompleted(client, vote2);
      console.log("🗳️ Voter 2 voted for Candidate 2 in Shard 2 ✅");

      const vote3 = await voter3.sendTransaction({
        to: voteShard3Address,
        functionName: "vote",
        args: [candidate3],
        abi: VoteShard.abi,
        feeCredit: convertEthToWei(0.001),
      });
      await waitTillCompleted(client, vote3);
      console.log("🗳️ Voter 3 voted for Candidate 3 in Shard 3 ✅");

      const vote4 = await voter4.sendTransaction({
        to: voteShard4Address,
        functionName: "vote",
        args: [candidate1],
        abi: VoteShard.abi,
        feeCredit: convertEthToWei(0.001),
      });
      await waitTillCompleted(client, vote4);
      console.log("🗳️ Voter 4 voted for Candidate 1 in Shard 4 ✅");

      console.log("🛑 Waiting for voting to end...");
      await waitUntilDbTimestamp(client, endTime);

      console.log("📊 Fetching results from shards...");
      const tallyShard1 = (await voteShard1Contract.read.tallyVotes(
        []
      )) as number[];
      const tallyShard2 = (await voteShard2Contract.read.tallyVotes(
        []
      )) as number[];
      const tallyShard3 = (await voteShard3Contract.read.tallyVotes(
        []
      )) as number[];
      const tallyShard4 = (await voteShard4Contract.read.tallyVotes(
        []
      )) as number[];

      console.log("📦 Results from each shard");
      console.log(
        `🧩 Shard 1: C1=${tallyShard1[1]}, C2=${tallyShard1[2]}, C3=${tallyShard1[3]}`
      );
      console.log(
        `🧩 Shard 2: C1=${tallyShard2[1]}, C2=${tallyShard2[2]}, C3=${tallyShard2[3]}`
      );
      console.log(
        `🧩 Shard 3: C1=${tallyShard3[1]}, C2=${tallyShard3[2]}, C3=${tallyShard3[3]}`
      );
      console.log(
        `🧩 Shard 4: C1=${tallyShard4[1]}, C2=${tallyShard4[2]}, C3=${tallyShard4[3]}`
      );

      console.log(
        "🧮 Calculating final tally across all shards using VoteManager..."
      );

      const tallyShardsVotesResponse = await deployerWallet.sendTransaction({
        to: voteManagerAddress,
        functionName: "tallyTotalVotes",
        abi: VoteManager.abi as Abi,
        feeCredit: convertEthToWei(0.001),
        value: 5000n,
      });

      await waitTillCompleted(client, tallyShardsVotesResponse);

      await new Promise((res) => setTimeout(res, 10000));

      const totalTally = (await voteManagerContract.read.getVotingResult(
        []
      )) as number[];

      console.log(
        `📊 Final Aggregated Results: C1=${totalTally[1]}, C2=${totalTally[2]}, C3=${totalTally[3]}`
      );

      console.log("🎉 End-to-end voting test completed!");
    } catch (e: unknown) {
      console.error("❌ Error occurred:");
      if (e instanceof Error) {
        console.error(e.message);
      } else {
        console.error("Unknown error occurred during execution.");
      }
    }
  }
);
