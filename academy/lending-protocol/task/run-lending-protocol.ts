import {
  FaucetClient,
  HttpTransport,
  PublicClient,
  convertEthToWei,
  generateSmartAccount,
  getContract,
  waitTillCompleted,
} from "@nilfoundation/niljs";

import { type Abi, decodeFunctionResult, encodeFunctionData } from "viem";

import * as dotenv from "dotenv";
import { task } from "hardhat/config";
dotenv.config();

task(
  "run-lending-protocol",
  "End to end test for the interaction page",
).setAction(async () => {
  // Import the compiled contracts
  const GlobalLedger = require("../artifacts/contracts/CollateralManager.sol/GlobalLedger.json");
  const InterestManager = require("../artifacts/contracts/InterestManager.sol/InterestManager.json");
  const LendingPool = require("../artifacts/contracts/LendingPool.sol/LendingPool.json");
  const Oracle = require("../artifacts/contracts/Oracle.sol/Oracle.json");

  // Initialize the PublicClient to interact with the blockchain
  const client = new PublicClient({
    transport: new HttpTransport({
      endpoint: process.env.NIL_RPC_ENDPOINT as string,
    }),
  });
  const listOfShards = await client.getShardIdList();
  console.log("List of shards:", listOfShards);

  // Initialize the FaucetClient to top up accounts with test tokens
  const faucet = new FaucetClient({
    transport: new HttpTransport({
      endpoint: process.env.NIL_RPC_ENDPOINT as string,
    }),
  });

  console.log("Faucet client created");

  // Deploying a new smart account for the deployer
  console.log("Deploying Wallet");
  const deployerWallet = await generateSmartAccount({
    shardId: listOfShards[0],
    rpcEndpoint: process.env.NIL_RPC_ENDPOINT as string,
    faucetEndpoint: process.env.NIL_RPC_ENDPOINT as string,
  });

  console.log(`Deployer smart account generated at ${deployerWallet.address}`);

  // Top up the deployer's smart account with USDT for contract deployment
  const topUpSmartAccount = await faucet.topUpAndWaitUntilCompletion(
    {
      smartAccountAddress: deployerWallet.address,
      faucetAddress: process.env.USDT as `0x${string}`,
      amount: BigInt(3000),
    },
    client,
  );

  console.log(
    `Deployer smart account ${deployerWallet.address} has been topped up with 3000 USDT at tx hash ${topUpSmartAccount}`,
  );

  // Deploy InterestManager contract on second shard
  const { address: deployInterestManager, hash: deployInterestManagerHash } =
    await deployerWallet.deployContract({
      shardId: listOfShards[1],
      args: [],
      bytecode: InterestManager.bytecode as `0x${string}`,
      abi: InterestManager.abi as Abi,
      salt: BigInt(Math.floor(Math.random() * 10000)),
    });

  await waitTillCompleted(client, deployInterestManagerHash);
  console.log(
    `Interest Manager deployed at ${deployInterestManager} with hash ${deployInterestManagerHash} on shard ${listOfShards[1]}`,
  );

  // Deploy GlobalLedger contract on third shard
  const { address: deployGlobalLedger, hash: deployGlobalLedgerHash } =
    await deployerWallet.deployContract({
      shardId: listOfShards[2],
      args: [],
      bytecode: GlobalLedger.bytecode as `0x${string}`,
      abi: GlobalLedger.abi as Abi,
      salt: BigInt(Math.floor(Math.random() * 10000)),
    });

  await waitTillCompleted(client, deployGlobalLedgerHash);
  console.log(
    `Global Ledger deployed at ${deployGlobalLedger} with hash ${deployGlobalLedgerHash} on shard ${listOfShards[2]}`,
  );

  // Deploy Oracle contract on fourth shard
  const { address: deployOracle, hash: deployOracleHash } =
    await deployerWallet.deployContract({
      shardId: listOfShards[3],
      args: [],
      bytecode: Oracle.bytecode as `0x${string}`,
      abi: Oracle.abi as Abi,
      salt: BigInt(Math.floor(Math.random() * 10000)),
    });

  await waitTillCompleted(client, deployOracleHash);
  console.log(
    `Oracle deployed at ${deployOracle} with hash ${deployOracleHash} on shard ${listOfShards[3]}`,
  );

  // Deploy LendingPool contracts on all shards
  const lendingPools: { address: `0x${string}`; shardId: number }[] = [];

  for (const shardId of listOfShards) {
    const { address: deployLendingPool, hash: deployLendingPoolHash } =
      await deployerWallet.deployContract({
        shardId,
        args: [
          deployGlobalLedger,
          deployInterestManager,
          deployOracle,
          process.env.USDT as `0x${string}`,
          process.env.ETH as `0x${string}`,
        ],
        bytecode: LendingPool.bytecode as `0x${string}`,
        abi: LendingPool.abi as Abi,
        salt: BigInt(Math.floor(Math.random() * 10000)),
      });

    await waitTillCompleted(client, deployLendingPoolHash);
    console.log(
      `Lending Pool deployed at ${deployLendingPool} with hash ${deployLendingPoolHash} on shard ${shardId}`,
    );

    lendingPools.push({ address: deployLendingPool as `0x${string}`, shardId });
  }

  // Register lending pools with each other
  for (let i = 0; i < lendingPools.length; i++) {
    const currentPool = lendingPools[i];

    for (let j = 0; j < lendingPools.length; j++) {
      if (i !== j) {
        const otherPool = lendingPools[j];

        const registerCallData = encodeFunctionData({
          abi: LendingPool.abi as Abi,
          functionName: "registerLendingPool",
          args: [otherPool.address],
        });

        const registerResponse = await deployerWallet.sendTransaction({
          to: currentPool.address,
          data: registerCallData,
        });

        await waitTillCompleted(client, registerResponse);
        console.log(
          `Registered lending pool ${otherPool.address} (shard ${otherPool.shardId}) with ${currentPool.address} (shard ${currentPool.shardId}) at tx hash ${registerResponse}`,
        );
      }
    }
  }

  // Generate two smart accounts (account1 and account2)
  const account1 = await generateSmartAccount({
    shardId: listOfShards[0],
    rpcEndpoint: process.env.NIL_RPC_ENDPOINT as string,
    faucetEndpoint: process.env.NIL_RPC_ENDPOINT as string,
  });

  console.log(`Account 1 generated at ${account1.address}`);

  const account2 = await generateSmartAccount({
    shardId: listOfShards[2],
    rpcEndpoint: process.env.NIL_RPC_ENDPOINT as string,
    faucetEndpoint: process.env.NIL_RPC_ENDPOINT as string,
  });

  console.log(`Account 2 generated at ${account2.address}`);

  // Top up account1 with NIL, USDT, and ETH for testing
  const topUpAccount1 = await faucet.topUpAndWaitUntilCompletion(
    {
      smartAccountAddress: account1.address,
      faucetAddress: process.env.NIL as `0x${string}`,
      amount: BigInt(1),
    },
    client,
  );

  const topUpAccount1WithUSDT = await faucet.topUpAndWaitUntilCompletion(
    {
      smartAccountAddress: account1.address,
      faucetAddress: process.env.USDT as `0x${string}`,
      amount: BigInt(30),
    },
    client,
  );

  const topUpAccount1WithETH = await faucet.topUpAndWaitUntilCompletion(
    {
      smartAccountAddress: account1.address,
      faucetAddress: process.env.ETH as `0x${string}`,
      amount: BigInt(10),
    },
    client,
  );

  console.log(`Account 1 topped up with 1 NIL at tx hash ${topUpAccount1}`);
  console.log(
    `Account 1 topped up with 30 USDT at tx hash ${topUpAccount1WithUSDT}`,
  );
  console.log(
    `Account 1 topped up with 10 ETH at tx hash ${topUpAccount1WithETH}`,
  );

  // Top up account2 with ETH for testing
  const topUpAccount2 = await faucet.topUpAndWaitUntilCompletion(
    {
      smartAccountAddress: account2.address,
      faucetAddress: process.env.ETH as `0x${string}`,
      amount: BigInt(5),
    },
    client,
  );

  console.log(`Account 2 topped up with 5 ETH at tx hash ${topUpAccount2}`);

  // Log the token balances of account1 and account2
  console.log(
    "Tokens in account 1:",
    await client.getTokens(account1.address, "latest"),
  );
  console.log(
    "Tokens in account 2:",
    await client.getTokens(account2.address, "latest"),
  );

  // Set the price for USDT and ETH in the Oracle contract
  const setUSDTPrice = encodeFunctionData({
    abi: Oracle.abi as Abi,
    functionName: "setPrice",
    args: [process.env.USDT, 1n],
  });

  const setETHPrice = encodeFunctionData({
    abi: Oracle.abi as Abi,
    functionName: "setPrice",
    args: [process.env.ETH, 2n],
  });

  // Set the price for USDT
  const setOraclePriceUSDT = await deployerWallet.sendTransaction({
    to: deployOracle,
    data: setUSDTPrice,
  });

  await waitTillCompleted(client, setOraclePriceUSDT);
  console.log(`Oracle price set for USDT at tx hash ${setOraclePriceUSDT}`);

  // Set the price for ETH
  const setOraclePriceETH = await deployerWallet.sendTransaction({
    to: deployOracle,
    data: setETHPrice,
  });

  await waitTillCompleted(client, setOraclePriceETH);
  console.log(`Oracle price set for ETH at tx hash ${setOraclePriceETH}`);

  // Retrieve the prices of USDT and ETH from the Oracle contract
  const usdtPriceRequest = await client.call(
    {
      from: deployOracle,
      to: deployOracle,
      data: encodeFunctionData({
        abi: Oracle.abi as Abi,
        functionName: "getPrice",
        args: [process.env.USDT],
      }),
    },
    "latest",
  );

  const ethPriceRequest = await client.call(
    {
      from: deployOracle,
      to: deployOracle,
      data: encodeFunctionData({
        abi: Oracle.abi as Abi,
        functionName: "getPrice",
        args: [process.env.ETH],
      }),
    },
    "latest",
  );

  const usdtPrice = decodeFunctionResult({
    abi: Oracle.abi as Abi,
    functionName: "getPrice",
    data: usdtPriceRequest.data,
  });

  const ethPrice = decodeFunctionResult({
    abi: Oracle.abi as Abi,
    functionName: "getPrice",
    data: ethPriceRequest.data,
  });

  console.log(`Price of USDT is ${usdtPrice}`);
  console.log(`Price of ETH is ${ethPrice}`);

  // Perform a deposit of USDT by account1 into the LendingPool on shard 1
  const depositUSDT = {
    id: process.env.USDT as `0x${string}`,
    amount: 12n,
  };

  const depositUSDTResponse = await account1.sendTransaction({
    to: lendingPools[0].address,
    functionName: "deposit",
    abi: LendingPool.abi as Abi,
    tokens: [depositUSDT],
    feeCredit: convertEthToWei(0.001),
  });

  await waitTillCompleted(client, depositUSDTResponse);
  console.log(
    `Account 1 deposited 12 USDT at tx hash ${depositUSDTResponse}`,
  );

  // Perform a deposit of ETH by account2 into the LendingPool on shard 3
  const depositETH = {
    id: process.env.ETH as `0x${string}`,
    amount: 5n,
  };

  const depositETHResponse = await account2.sendTransaction({
    to: lendingPools[2].address,
    functionName: "deposit",
    abi: LendingPool.abi as Abi,
    tokens: [depositETH],
    feeCredit: convertEthToWei(0.001),
  });

  await waitTillCompleted(client, depositETHResponse);
  console.log(`Account 2 deposited 5 ETH at tx hash ${depositETHResponse}`);

  // Retrieve the deposit balances of account1 and account2 from GlobalLedger
  const globalLedgerContract = getContract({
    client,
    abi: GlobalLedger.abi,
    address: deployGlobalLedger,
  });

  const account1Balance = await globalLedgerContract.read.getDeposit([
    account1.address,
    process.env.USDT,
  ]);
  const account2Balance = await globalLedgerContract.read.getDeposit([
    account2.address,
    process.env.ETH,
  ]);

  console.log(`Account 1 balance in global ledger is ${account1Balance}`);
  console.log(`Account 2 balance in global ledger is ${account2Balance}`);

  // Perform a cross-shard borrow operation by account1 for 5 ETH
  const borrowETH = encodeFunctionData({
    abi: LendingPool.abi as Abi,
    functionName: "borrow",
    args: [5, process.env.ETH],
  });

  const account1BalanceBeforeBorrow = await client.getTokens(
    account1.address,
    "latest",
  );
  console.log("Account 1 balance before borrow:", account1BalanceBeforeBorrow);

  const borrowETHResponse = await account1.sendTransaction({
    to: lendingPools[0].address,
    data: borrowETH,
    feeCredit: convertEthToWei(0.001),
  });

  await waitTillCompleted(client, borrowETHResponse);
  console.log(`Account 1 borrowed 5 ETH at tx hash ${borrowETHResponse}`);

  const account1BalanceAfterBorrow = await client.getTokens(
    account1.address,
    "latest",
  );
  console.log("Account 1 balance after borrow:", account1BalanceAfterBorrow);

  // Top up account1 with NIL for loan repayment to avoid insufficient balance
  const topUpSmartAccount1WithNIL = await faucet.topUpAndWaitUntilCompletion(
    {
      smartAccountAddress: account1.address,
      faucetAddress: process.env.NIL as `0x${string}`,
      amount: BigInt(1),
    },
    client,
  );

  await waitTillCompleted(client, topUpSmartAccount1WithNIL);
  console.log(
    `Account 1 topped up with 1 NIL at tx hash ${topUpSmartAccount1WithNIL}`,
  );

  const account1BalanceAfterTopUp = await client.getBalance(account1.address);
  console.log("Account 1 balance after top up:", account1BalanceAfterTopUp);

  // Perform a loan repayment by account1
  const repayETH = [
    {
      id: process.env.ETH as `0x${string}`,
      amount: BigInt(6),
    },
  ];

  const repayETHData = encodeFunctionData({
    abi: LendingPool.abi as Abi,
    functionName: "repayLoan",
    args: [],
  });

  const repayETHResponse = await account1.sendTransaction({
    to: lendingPools[0].address,
    data: repayETHData,
    tokens: repayETH,
    feeCredit: convertEthToWei(0.001),
  });

  await waitTillCompleted(client, repayETHResponse);
  console.log(`Account 1 repaid 6 ETH at tx hash ${repayETHResponse}`);

  const account1BalanceAfterRepay = await client.getTokens(
    account1.address,
    "latest",
  );
  console.log("Account 1 balance after repay:", account1BalanceAfterRepay);
});
