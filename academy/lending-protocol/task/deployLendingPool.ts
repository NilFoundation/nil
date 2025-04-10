import {
  HttpTransport,
  PublicClient,
  FaucetClient,
  generateSmartAccount,
  waitTillCompleted,
} from "@nilfoundation/niljs";
import { encodeFunctionData, decodeEventLog } from "viem";
import * as dotenv from "dotenv";

dotenv.config();

const LendingPoolFactory = require("../artifacts/contracts/LendingPoolFactory.sol/LendingPoolFactory.json");
const GlobalLedger = require("../artifacts/contracts/CollateralManager.sol/GlobalLedger.json");
const InterestManager = require("../artifacts/contracts/InterestManager.sol/InterestManager.json");
const Oracle = require("../artifacts/contracts/Oracle.sol/Oracle.json");

const endpoint = process.env.NIL_RPC_ENDPOINT as string;

const shardFactory = 1;
const shardLedger = 3;
const shardInterest = 2;
const shardOracle = 4;

interface LendingPoolDeployedEvent {
  eventName: "LendingPoolDeployed";
  args: {
    pool: string;
    shardId: number;
  };
}

async function main() {
  const client = new PublicClient({ transport: new HttpTransport({ endpoint }) });
  const faucet = new FaucetClient({ transport: new HttpTransport({ endpoint }) });

  const deployer = await generateSmartAccount({
    shardId: shardFactory,
    rpcEndpoint: endpoint,
    faucetEndpoint: endpoint,
  });

  console.log(" Smart account:", deployer.address);

  await faucet.topUpAndWaitUntilCompletion(
    {
      smartAccountAddress: deployer.address,
      faucetAddress: process.env.USDT as `0x${string}`,
      amount: BigInt(5000),
    },
    client
  );

  const deployWithSalt = async (shardId: number, bytecode: string, abi: any, args: any[] = []) => {
    // Ensure the deploy function returns an object with both `address` and `hash` correctly typed
    const result = await deployer.deployContract({
      shardId,
      args,
      bytecode: bytecode as `0x${string}`,
      abi,
      salt: BigInt(Math.floor(Math.random() * 1e6)),
    });
    
    // Explicitly return both `address` and `hash`
    return { address: result.address, hash: result.hash };
  };

  const interestManager = await deployWithSalt(shardInterest, InterestManager.bytecode, InterestManager.abi);
  const globalLedger = await deployWithSalt(shardLedger, GlobalLedger.bytecode, GlobalLedger.abi);
  const oracle = await deployWithSalt(shardOracle, Oracle.bytecode, Oracle.abi);

  const { address: factoryAddress, hash: factoryTx } = await deployer.deployContract({
    shardId: shardFactory,
    bytecode: LendingPoolFactory.bytecode as `0x${string}`,
    abi: LendingPoolFactory.abi,
    args: [
      globalLedger,
      shardLedger,
      interestManager,
      oracle,
      process.env.USDT,
      process.env.ETH,
    ],
    salt: BigInt(Date.now()),
  });

  await waitTillCompleted(client, factoryTx);
  console.log(" LendingPoolFactory deployed at:", factoryAddress);

  const deployCalldata = encodeFunctionData({
    abi: LendingPoolFactory.abi,
    functionName: "deployLendingPool",
    args: [],
  });

  const { hash: deployHash } = await deployer.sendTransaction({
    to: factoryAddress,
    data: deployCalldata,
    value: 0n,
  });

  await waitTillCompleted(client, deployHash);
  console.log(" LendingPool deployed via factory. Tx:", deployHash);

  const receipt = await client.getTransactionReceiptByHash(deployHash);

  if (!receipt) {
    throw new Error(" Transaction receipt not found. It may not have been finalized.");
  }

  // Decode the logs and assert the event type
  const log = receipt.logs
    .map((log) => {
      try {
        // Decode the log with the proper ABI
        const decodedLog = decodeEventLog({
          abi: LendingPoolFactory.abi,
          data: log.data as `0x${string}`,
          topics: log.topics as [`0x${string}`, ...`0x${string}`[]],
        });

        // Check if the decoded log contains the expected event name
        if (decodedLog && decodedLog.eventName === "LendingPoolDeployed") {
          return decodedLog as LendingPoolDeployedEvent; // Explicitly cast to the correct type
        }

        return null;
      } catch (e) {
        return null;
      }
    })
    .find((l) => l !== null); // Find the first non-null log

  if (log) {
    const pool = log.args.pool;
    const shard = log.args.shardId;
    console.log(`LendingPool deployed at: ${pool} on shard ${shard}`);
  } else {
    console.warn("LendingPoolDeployed event not found");
  }
}

main().catch(console.error);
