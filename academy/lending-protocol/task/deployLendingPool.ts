import {
  HttpTransport,
  PublicClient,
  FaucetClient,
  generateSmartAccount,
  waitTillCompleted,
  Contract,
} from "@nilfoundation/niljs";
import { task } from "hardhat/config";
import { config } from "dotenv";
import fs from "fs";
import * as path from "path";
import type { Abi } from "viem";

config();

task("deploy-lending-pool", "Deploy LendingPool with linked contracts and test").setAction(async () => {
  const LendingPool = require("../artifacts/contracts/LendingPool.sol/LendingPool.json");
  const GlobalLedger = require("../artifacts/contracts/CollateralManager.sol/GlobalLedger.json");
  const InterestManager = require("../artifacts/contracts/InterestManager.sol/InterestManager.json");
  const Oracle = require("../artifacts/contracts/Oracle.sol/Oracle.json");

  const endpoint = process.env.NIL_RPC_ENDPOINT as string;
  const client = new PublicClient({ transport: new HttpTransport({ endpoint }) });
  const faucet = new FaucetClient({ transport: new HttpTransport({ endpoint }) });

  // Step 1: Create deployer smart account
  const deployer = await generateSmartAccount({
    shardId: 1,
    rpcEndpoint: endpoint,
    faucetEndpoint: endpoint,
  });

  console.log(` Deployer Smart Account: ${deployer.address}`);

  const topUpHash = await faucet.topUpAndWaitUntilCompletion(
    {
      smartAccountAddress: deployer.address,
      faucetAddress: process.env.USDT as `0x${string}`,
      amount: BigInt(2000),
    },
    client
  );
  console.log(`Deployer funded with 2000 USDT. Tx: ${topUpHash}`);

  // Step 2: Deploy contracts
  const deployWithSalt = async (shardId: number, bytecode: string, abi: Abi) => {
    const { address, hash } = await deployer.deployContract({
      shardId,
      args: [],
      bytecode: bytecode as `0x${string}`,
      abi,
      salt: BigInt(Math.floor(Math.random() * 1000000)),
    });
    await waitTillCompleted(client, hash);
    return address;
  };

  const interestManager = await deployWithSalt(2, InterestManager.bytecode, InterestManager.abi);
  console.log(`InterestManager deployed: ${interestManager}`);

  const globalLedger = await deployWithSalt(3, GlobalLedger.bytecode, GlobalLedger.abi);
  console.log(`GlobalLedger deployed: ${globalLedger}`);

  const oracle = await deployWithSalt(4, Oracle.bytecode, Oracle.abi);
  console.log(`Oracle deployed: ${oracle}`);

  // Step 3: Deploy LendingPool with linked contracts
  const { address: lendingPool, hash: lendingPoolHash } = await deployer.deployContract({
    shardId: 1,
    args: [
      globalLedger,
      interestManager,
      oracle,
      process.env.USDT,
      process.env.ETH,
    ],
    bytecode: LendingPool.bytecode as `0x${string}`,
    abi: LendingPool.abi as Abi,
    salt: BigInt(Math.floor(Math.random() * 1000000)),
  });
  await waitTillCompleted(client, lendingPoolHash);
  console.log(`LendingPool deployed: ${lendingPool}`);

  // Step 4: Save deployed addresses to `.env`
  const envPath = path.resolve(__dirname, "../.env");
  const envData = fs.readFileSync(envPath, "utf-8");

  const updateEnvVar = (data: string, key: string, value: string) => {
    const regex = new RegExp(`^${key}=.*$`, "m");
    const line = `${key}=${value}`;
    return data.match(regex) ? data.replace(regex, line) : data + `\n${line}`;
  };

  let updatedEnv = envData;
  updatedEnv = updateEnvVar(updatedEnv, "LENDING_POOL", lendingPool);
  updatedEnv = updateEnvVar(updatedEnv, "GLOBAL_LEDGER", globalLedger);
  updatedEnv = updateEnvVar(updatedEnv, "INTEREST_MANAGER", interestManager);
  updatedEnv = updateEnvVar(updatedEnv, "ORACLE", oracle);

  fs.writeFileSync(envPath, updatedEnv);
  console.log(".env updated with deployed contract addresses.");

  // Step 5: (Optional) Create a test user smart account
  const testUser = await generateSmartAccount({
    shardId: 1,
    rpcEndpoint: endpoint,
    faucetEndpoint: endpoint,
  });

  console.log(`Test Smart Account: ${testUser.address}`);

  const testTopUp = await faucet.topUpAndWaitUntilCompletion(
    {
      smartAccountAddress: testUser.address,
      faucetAddress: process.env.USDT as `0x${string}`,
      amount: BigInt(500),
    },
    client
  );

  console.log(`Test user funded with 500 USDT. Tx: ${testTopUp}`);

  // Step 6: Contract verification test call (read public variable)
  const lendingPoolContract = new Contract({
    client,
    abi: LendingPool.abi,
    address: lendingPool,
  });

  try {
    const oracleAddr = await lendingPoolContract.read.oracle();
    console.log(`Verified LendingPool oracle address: ${oracleAddr}`);
  } catch (err) {
    console.error("Failed to verify LendingPool setup:", err);
  }

  // Summary
  console.log("\n Summary:");
  console.log(`Deployer Smart Account:  ${deployer.address}`);
  console.log(`Test User Smart Account: ${testUser.address}`);
  console.log(`LendingPool:             ${lendingPool}`);
  console.log(`GlobalLedger:            ${globalLedger}`);
  console.log(`InterestManager:         ${interestManager}`);
  console.log(`Oracle:                  ${oracle}`);
});
