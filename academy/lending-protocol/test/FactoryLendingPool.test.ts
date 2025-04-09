import {
  HttpTransport,
  PublicClient,
  FaucetClient,
  generateSmartAccount,
  waitTillCompleted,
} from "@nilfoundation/niljs";
import { describe, it, before } from "mocha";
import { expect } from "chai";
import * as dotenv from "dotenv";
import { encodeFunctionData, decodeEventLog } from "viem";
import type { Abi } from "viem";

dotenv.config();

const LendingPoolFactory = require("../artifacts/contracts/LendingPoolFactory.sol/LendingPoolFactory.json");
const LendingPool = require("../artifacts/contracts/LendingPool.sol/LendingPool.json");
const GlobalLedger = require("../artifacts/contracts/CollateralManager.sol/GlobalLedger.json");
const InterestManager = require("../artifacts/contracts/InterestManager.sol/InterestManager.json");
const Oracle = require("../artifacts/contracts/Oracle.sol/Oracle.json");

describe("LendingPoolFactory", function () {
  let client: PublicClient;
  let faucet: FaucetClient;
  let deployer: Awaited<ReturnType<typeof generateSmartAccount>>;
  let factoryAddress: string;

  const endpoint = process.env.NIL_RPC_ENDPOINT as string;
  const shardFactory = 1;
  const shardLedger = 3;
  const shardInterest = 2;
  const shardOracle = 4;

  before(async function () {
    this.timeout(600_000);

    client = new PublicClient({ transport: new HttpTransport({ endpoint }) });
    faucet = new FaucetClient({ transport: new HttpTransport({ endpoint }) });

    deployer = await generateSmartAccount({
      shardId: shardFactory,
      rpcEndpoint: endpoint,
      faucetEndpoint: endpoint,
    });

    console.log(`Deployer Smart Account: ${deployer.address}`);

    await faucet.topUpAndWaitUntilCompletion(
      {
        smartAccountAddress: deployer.address,
        faucetAddress: process.env.USDT as `0x${string}`,
        amount: BigInt(5000),
      },
      client
    );

    const deployWithSalt = async (
      shardId: number,
      bytecode: string,
      abi: Abi,
      args: any[] = []
    ) => {
      const { address, hash } = await deployer.deployContract({
        shardId,
        args,
        bytecode: bytecode as `0x${string}`,
        abi,
        salt: BigInt(Math.floor(Math.random() * 1000000)),
      });
      await waitTillCompleted(client, hash);
      return address;
    };

    const interestManager = await deployWithSalt(
      shardInterest,
      InterestManager.bytecode,
      InterestManager.abi
    );
    const globalLedger = await deployWithSalt(
      shardLedger,
      GlobalLedger.bytecode,
      GlobalLedger.abi
    );
    const oracle = await deployWithSalt(
      shardOracle,
      Oracle.bytecode,
      Oracle.abi
    );

    const { address: factory, hash } = await deployer.deployContract({
      shardId: shardFactory,
      bytecode: LendingPoolFactory.bytecode as `0x${string}`,
      abi: LendingPoolFactory.abi,
      args: [
        globalLedger,
        shardLedger, // pass the shard ID of globalLedger here
        interestManager,
        oracle,
        process.env.USDT,
        process.env.ETH,
      ],
      salt: BigInt(Date.now()),
    });

    await waitTillCompleted(client, hash);
    factoryAddress = factory;

    console.log("Factory deployed at:", factoryAddress);
  });

  it("should deploy a LendingPool via the factory and verify it", async function () {
    this.timeout(600_000);

    const data = encodeFunctionData({
      abi: LendingPoolFactory.abi,
      functionName: "deployLendingPool",
      args: [],
    });

    const { hash } = await deployer.execute({
      shardId: shardFactory,
      to: factoryAddress,
      data,
      value: 0n,
    });

    await waitTillCompleted(client, hash);
    console.log("LendingPool deployed via factory. Tx:", hash);

    // Get the receipt and decode logs
    const receipt = await client.getTransactionReceipt({ hash });

    const deploymentEvent = receipt.logs
      .map((log) => {
        try {
          return decodeEventLog({
            abi: LendingPoolFactory.abi,
            data: log.data,
            topics: log.topics,
          });
        } catch {
          return null;
        }
      })
      .find((log) => log?.eventName === "LendingPoolDeployed");

    expect(deploymentEvent).to.not.be.undefined;
    const deployedPoolAddress = deploymentEvent?.args?.pool;
    console.log("Deployed LendingPool:", deployedPoolAddress);

    expect(deployedPoolAddress).to.be.a("string").and.to.match(/^0x/);

    // Confirm via public getter
    const getterData = encodeFunctionData({
      abi: LendingPoolFactory.abi,
      functionName: "getDeployedPools",
      args: [],
    });

    const getterResult = await client.call({
      to: factoryAddress,
      data: getterData,
    });

    // You could decode result if ABI encoding is returned:
    // const pools = decodeFunctionResult({ abi, functionName, data: getterResult.data })

    console.log("getDeployedPools raw result:", getterResult.data);
  });
});
