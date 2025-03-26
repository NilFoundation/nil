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

import { type Abi, decodeFunctionResult, encodeFunctionData } from "viem";

import * as dotenv from "dotenv";
import { task } from "hardhat/config";
dotenv.config();

function getVotingTimestamps(blockTime: number) {
  const offsetInMinutes = 1;
  const durationInMinutes = 2;
  const startTime = blockTime + offsetInMinutes * 30;
  const endTime = startTime + durationInMinutes * 30;
  return { startTime, endTime };
}

type NilBlock<T extends boolean = false> = Block<T> & {
  dbTimestamp: number;
};

task("e2e", "End to end test for the interaction page").setAction(async () => {
  try {
    console.log("Starting e2e");

    const VoteManager = require("../artifacts/contracts/VoteManager.sol/VoteManager.json");
    const noOfShards = 2;
    const noOfChoices = 3;

    const VoteShard = require("../artifacts/contracts/VoteShard.sol/VoteShard.json");

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

    console.log("Faucet client created");

    // Generate DeployerWallet contract on shard 1
    const deployerWallet = await generateSmartAccount({
      shardId: 1,
      rpcEndpoint: process.env.NIL_RPC_ENDPOINT as string,
      faucetEndpoint: process.env.NIL_RPC_ENDPOINT as string,
    });

    const balance = await deployerWallet.getBalance();

    console.log(
      `Deployer smart account generated at ${deployerWallet.address} with bal: ${balance} NIL`
    );

    // Top up the deployer's smart account with NIL for contract deployment
    const topUpSmartAccount = await faucet.topUpAndWaitUntilCompletion(
      {
        smartAccountAddress: deployerWallet.address,
        faucetAddress: process.env.NIL as `0x${string}`,
        amount: convertEthToWei(1000),
      },
      client
    );

    console.log(
      `Deployer smart account ${deployerWallet.address} has been topped up with 1000 NIL at tx hash ${topUpSmartAccount}`
    );

    const block = (await client.getBlockByNumber(
      "latest",
      false,
      1
    )) as NilBlock;

    const { startTime, endTime } = getVotingTimestamps(block.dbTimestamp);

    console.log(block.dbTimestamp);

    // Deploy VoteManager contract on shard 1
    const { address: voteManagerAddress, hash: deployVoteProtocolHash } =
      await deployerWallet.deployContract({
        shardId: 2,
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

    if (txReceipt && !txReceipt?.success) {
      throw new Error(
        `VoteManager Deployment Failed.\nReason: ${txReceipt.errorMessage}\nStatus: ${txReceipt.status}`
      );
    }

    console.log(
      `Vote Manager Pool deployed at ${voteManagerAddress} with hash ${deployVoteProtocolHash} on shard 1`
    );

    const voteManagerContract = getContract({
      client,
      abi: VoteManager.abi,
      address: voteManagerAddress,
    });

    const deployShardsResponse = await deployerWallet.sendTransaction({
      to: voteManagerAddress,
      functionName: "deployVotingShards",
      abi: VoteManager.abi as Abi,
      feeCredit: convertEthToWei(0.001),
    });

    await waitTillCompleted(client, deployShardsResponse);

    txReceipt = await client.getTransactionReceiptByHash(deployShardsResponse);

    if (txReceipt && !txReceipt.success) {
      throw new Error(
        `Shard deployment failed.\nReason: ${txReceipt.errorMessage}\nStatus: ${txReceipt.status}`
      );
    }

    console.log(`Voting shards deployed tx hash ${deployShardsResponse}`);

    const voteShard1Address = (await voteManagerContract.read.getShardAddress([
      1,
    ])) as `0x${string}`;
    const voteShard2Address = (await voteManagerContract.read.getShardAddress([
      2,
    ])) as `0x${string}`;
    const voteShard3Address = (await voteManagerContract.read.getShardAddress([
      3,
    ])) as `0x${string}`;
    const voteShard4Address = (await voteManagerContract.read.getShardAddress([
      4,
    ])) as `0x${string}`;

    const voteShard1Contract = getContract({
      client,
      abi: VoteShard.abi,
      address: voteShard1Address,
    });

    const voteShard2Contract = getContract({
      client,
      abi: VoteShard.abi,
      address: voteShard1Address,
    });

    // const voteShard1Contract = getContract({
    //   client,
    //   abi: VoteShard.abi,
    //   address: voteShard1Address as `0x${string}`,
    // });

    // const voteShard1Contract = getContract({
    //   client,
    //   abi: VoteShard.abi,
    //   address: voteShard1Address as `0x${string}`,
    // });

    console.log("Vote Shards Addresses");
    console.log(`Shard 1: ${voteShard1Address}`);
    console.log(`Shard 2: ${voteShard2Address}`);
    console.log(`Shard 3: ${voteShard3Address}`);
    console.log(`Shard 4: ${voteShard4Address}`);

    console.log("Generating Voter Accounts");

    const account1 = await generateSmartAccount({
      shardId: 1,
      rpcEndpoint: process.env.NIL_RPC_ENDPOINT as string,
      faucetEndpoint: process.env.NIL_RPC_ENDPOINT as string,
    });

    const account2 = await generateSmartAccount({
      shardId: 2,
      rpcEndpoint: process.env.NIL_RPC_ENDPOINT as string,
      faucetEndpoint: process.env.NIL_RPC_ENDPOINT as string,
    });

    // const account3 = await generateSmartAccount({
    //   shardId: 3,
    //   rpcEndpoint: process.env.NIL_RPC_ENDPOINT as string,
    //   faucetEndpoint: process.env.NIL_RPC_ENDPOINT as string,
    // });

    // const account4 = await generateSmartAccount({
    //   shardId: 4,
    //   rpcEndpoint: process.env.NIL_RPC_ENDPOINT as string,
    //   faucetEndpoint: process.env.NIL_RPC_ENDPOINT as string,
    // });

    // Vote On Shards
    console.log("Vote on Shard 1");
    const firstVoteResponse = await account1.sendTransaction({
      to: voteShard1Address,
      functionName: "vote",
      args: [1, account1.address],
      abi: VoteShard.abi as Abi,
      feeCredit: convertEthToWei(0.001),
    });

    waitTillCompleted(client, firstVoteResponse);

    txReceipt = await client.getTransactionReceiptByHash(firstVoteResponse);

    if (txReceipt && !txReceipt.success) {
      throw new Error(
        `First Vote actions failed.\nReason: ${txReceipt.errorMessage}\nStatus: ${txReceipt.status}`
      );
    }

    console.log(txReceipt);

    // console.log(firstVoteResponse);
    // console.log("Vote on Shard 2");
    // // client.setShardId(2);
    // const secondVoteResponse = await account2.sendTransaction({
    //   to: voteShard1Address,
    //   functionName: "vote",
    //   args: [1, account2.address],
    //   abi: VoteShard.abi as Abi,
    //   feeCredit: convertEthToWei(0.001),
    // });

    // waitTillCompleted(client, secondVoteResponse);

    // txReceipt = await client.getTransactionReceiptByHash(secondVoteResponse);

    // if (txReceipt && !txReceipt.success) {
    //   throw new Error(
    //     `First Vote actions failed.\nReason: ${txReceipt.errorMessage}\nStatus: ${txReceipt.status}`
    //   );
    // }

    // console.log(secondVoteResponse);

    // await faucet.topUpAndWaitUntilCompletion(
    //   {
    //     smartAccountAddress: deployerWallet.address,
    //     faucetAddress: process.env.NIL as `0x${string}`,
    //     amount: convertEthToWei(1000),
    //   },
    //   client
    // );

    // const tallyShardsVotesResponse = await deployerWallet.sendTransaction({
    //   to: voteManagerAddress,
    //   functionName: "tallyTotalVotes",
    //   abi: VoteManager.abi as Abi,
    //   feeCredit: convertEthToWei(0.001),
    //   value: 5000n,
    // });

    // console.log(tallyShardsVotesResponse);

    // // await waitTillCompleted(client, tallyShardsVotesResponse);

    // txReceipt = await client.getTransactionReceiptByHash(
    //   tallyShardsVotesResponse
    // );
    // console.log(txReceipt);
    // if (txReceipt && !txReceipt.success) {
    //   throw new Error(
    //     `Tally votes actions failed.\nReason: ${txReceipt.errorMessage}\nStatus: ${txReceipt.status}`
    //   );
    // }
  } catch (e: unknown) {
    console.error(e);
    if (e instanceof Error) {
      console.error("Error:", e.message);
    } else {
      console.error("Unknown error occurred during execution.");
    }
  }
});
