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
    shardId: 1,
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

  // Deploy InterestManager contract on shard 2
  const { address: deployInterestManager, tx: deployInterestManagerTx } =
    await deployerWallet.deployContract({
      shardId: 2,
      args: [],
      bytecode: InterestManager.bytecode as `0x${string}`,
      abi: InterestManager.abi as Abi,
      salt: BigInt(Math.floor(Math.random() * 10000)),
    });

  await deployInterestManagerTx.wait();
  console.log(
    `Interest Manager deployed at ${deployInterestManager} with hash ${deployInterestManagerTx.hash} on shard 2`,
  );

  // Deploy GlobalLedger contract on shard 3
  const { address: deployGlobalLedger, tx: deployGlobalLedgerTx } =
    await deployerWallet.deployContract({
      shardId: 3,
      args: [],
      bytecode: GlobalLedger.bytecode as `0x${string}`,
      abi: GlobalLedger.abi as Abi,
      salt: BigInt(Math.floor(Math.random() * 10000)),
    });

  await deployGlobalLedgerTx.wait();
  console.log(
    `Global Ledger deployed at ${deployGlobalLedger} with hash ${deployGlobalLedgerTx.hash} on shard 3`,
  );

  // Deploy Oracle contract on shard 4
  const { address: deployOracle, tx: deployOracleTx } =
    await deployerWallet.deployContract({
      shardId: 4,
      args: [],
      bytecode: Oracle.bytecode as `0x${string}`,
      abi: Oracle.abi as Abi,
      salt: BigInt(Math.floor(Math.random() * 10000)),
    });

  await deployOracleTx.wait();
  console.log(
    `Oracle deployed at ${deployOracle} with hash ${deployOracleTx.hash} on shard 4`,
  );

  // Deploy LendingPool contract on shard 1, linking all other contracts
  const { address: deployLendingPool, tx: deployLendingPoolTx } =
    await deployerWallet.deployContract({
      shardId: 1,
      args: [
        deployGlobalLedger,
        deployInterestManager,
        deployOracle,
        process.env.USDT,
        process.env.ETH,
      ],
      bytecode: LendingPool.bytecode as `0x${string}`,
      abi: LendingPool.abi as Abi,
      salt: BigInt(Math.floor(Math.random() * 10000)),
    });

  await deployLendingPoolTx.wait();
  console.log(
    `Lending Pool deployed at ${deployLendingPool} with hash ${deployLendingPoolTx.hash} on shard 1`,
  );

  // Generate two smart accounts (account1 and account2)
  const account1 = await generateSmartAccount({
    shardId: 1,
    rpcEndpoint: process.env.NIL_RPC_ENDPOINT as string,
    faucetEndpoint: process.env.NIL_RPC_ENDPOINT as string,
  });

  console.log(`Account 1 generated at ${account1.address}`);

  const account2 = await generateSmartAccount({
    shardId: 3,
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

  await setOraclePriceUSDT.wait();
  console.log(
    `Oracle price set for USDT at tx hash ${setOraclePriceUSDT.hash}`,
  );

  // Set the price for ETH
  const setOraclePriceETH = await deployerWallet.sendTransaction({
    to: deployOracle,
    data: setETHPrice,
  });

  await setOraclePriceETH.wait();
  console.log(`Oracle price set for ETH at tx hash ${setOraclePriceETH.hash}`);

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

  // Perform a deposit of USDT by account1 into the LendingPool
  const depositUSDT = {
    id: process.env.USDT as `0x${string}`,
    amount: 12n,
  };

  const depositUSDTResponse = await account1.sendTransaction({
    to: deployLendingPool,
    functionName: "deposit",
    abi: LendingPool.abi as Abi,
    tokens: [depositUSDT],
    feeCredit: convertEthToWei(0.001),
  });

  await waitTillCompleted(client, depositUSDTResponse);
  console.log(`Account 1 deposited 12 USDT at tx hash ${depositUSDTResponse}`);

  // Perform a deposit of ETH by account2 into the LendingPool
  const depositETH = {
    id: process.env.ETH as `0x${string}`,
    amount: 5n,
  };

  const depositETHResponse = await account2.sendTransaction({
    to: deployLendingPool,
    functionName: "deposit",
    abi: LendingPool.abi as Abi,
    tokens: [depositETH],
    feeCredit: convertEthToWei(0.001),
  });

  await depositETHResponse.wait();
  console.log(
    `Account 2 deposited 1 ETH at tx hash ${depositETHResponse.hash}`,
  );

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

  // Perform a borrow operation by account1 for 1 ETH
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
    to: deployLendingPool,
    data: borrowETH,
    feeCredit: convertEthToWei(0.001),
  });

  await borrowETHResponse.wait();
  console.log(`Account 1 borrowed 5 ETH at tx hash ${borrowETHResponse.hash}`);

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
    to: deployLendingPool,
    data: repayETHData,
    tokens: repayETH,
    feeCredit: convertEthToWei(0.001),
  });

  await repayETHResponse.wait();
  console.log(`Account 1 repaid 1 ETH at tx hash ${repayETHResponse.hash}`);

  const account1BalanceAfterRepay = await client.getTokens(
    account1.address,
    "latest",
  );
  console.log("Account 1 balance after repay:", account1BalanceAfterRepay);
});
