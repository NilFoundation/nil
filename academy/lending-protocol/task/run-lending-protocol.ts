import {
  FaucetClient,
  HttpTransport,
  PublicClient,
  convertEthToWei,
  generateSmartAccount,
  getContract,
  waitTillCompleted,
  Transaction,
} from "@nilfoundation/niljs";

import { type Abi, decodeFunctionResult, encodeFunctionData } from "viem";

import * as dotenv from "dotenv";
import { task } from "hardhat/config";
dotenv.config();

// Define type for token info
interface TokenInfo {
  id: `0x${string}`;
  amount: bigint;
}

// Helper function to convert token record to array
function tokensRecordToArray(record: Record<string, bigint>): TokenInfo[] {
  return Object.entries(record).map(([id, amount]) => ({ id: id as `0x${string}`, amount }));
}

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
  const { address: deployInterestManager, tx: deployInterestManagerTx } =
    await deployerWallet.deployContract({
      shardId: listOfShards[1],
      args: [],
      bytecode: InterestManager.bytecode as `0x${string}`,
      abi: InterestManager.abi as Abi,
      salt: BigInt(Math.floor(Math.random() * 10000)),
    });

  await deployInterestManagerTx.wait();
  console.log(
    `Interest Manager deployed at ${deployInterestManager} with hash ${deployInterestManagerTx.hash} on shard ${listOfShards[1]}`,
  );

  // Deploy Oracle contract on fourth shard
  const { address: deployOracle, tx: deployOracleTx } =
    await deployerWallet.deployContract({
      shardId: listOfShards[3],
      args: [],
      bytecode: Oracle.bytecode as `0x${string}`,
      abi: Oracle.abi as Abi,
      salt: BigInt(Math.floor(Math.random() * 10000)),
    });

  await deployOracleTx.wait();
  console.log(
    `Oracle deployed at ${deployOracle} with hash ${deployOracleTx.hash} on shard ${listOfShards[3]}`,
  );

  // Deploy GlobalLedger (CentralLedger) contract on third shard
  const { address: deployGlobalLedger, tx: deployGlobalLedgerTx } =
    await deployerWallet.deployContract({
      shardId: listOfShards[2],
      args: [
        deployInterestManager,
        deployOracle,
        process.env.USDT as `0x${string}`,
        process.env.ETH as `0x${string}`,
      ],
      bytecode: GlobalLedger.bytecode as `0x${string}`,
      abi: GlobalLedger.abi as Abi,
      salt: BigInt(Math.floor(Math.random() * 10000)),
    });

  await deployGlobalLedgerTx.wait();
  console.log(
    `Global Ledger (Central) deployed at ${deployGlobalLedger} with hash ${deployGlobalLedgerTx.hash} on shard ${listOfShards[2]}`,
  );

  // Deploy LendingPool contracts via GlobalLedger
  console.log("\\nDeploying LendingPool contracts via GlobalLedger...");
  const deployPoolsTx = await deployerWallet.sendTransaction({
    to: deployGlobalLedger,
    functionName: "deployLendingPools",
    args: [listOfShards],
    abi: GlobalLedger.abi as Abi,
    // No value or tokens needed for this call
  });
  await deployPoolsTx.wait();
  console.log(`LendingPool deployment initiated via GlobalLedger at tx hash ${deployPoolsTx.hash}`);

  // Wait for async deployments to likely complete
  console.log("Waiting a few seconds for LendingPools to deploy asynchronously...");
  await new Promise(resolve => setTimeout(resolve, 8000)); // Adjust wait time if needed

  // Fetch deployed LendingPool addresses from GlobalLedger state
  console.log("Fetching deployed LendingPool addresses...");
  const lendingPools: { address: `0x${string}`; shardId: number }[] = [];
  const globalLedgerReader = getContract({ client, abi: GlobalLedger.abi, address: deployGlobalLedger });

  for (const shardId of listOfShards) {
    try {
      const poolAddress = await globalLedgerReader.read.lendingPoolsByShard([shardId]) as `0x${string}`;
      if (poolAddress && poolAddress !== '0x0000000000000000000000000000000000000000') {
        console.log(`  Found LendingPool on shard ${shardId} at address: ${poolAddress}`);
        lendingPools.push({ address: poolAddress, shardId });
      } else {
        console.log(`  No LendingPool found registered for shard ${shardId}.`);
      }
    } catch (error) {
      console.error(`Error fetching pool address for shard ${shardId}:`, error);
    }
  }

  if (lendingPools.length !== listOfShards.length) {
    console.warn(`WARNING: Expected ${listOfShards.length} pools, but only found ${lendingPools.length} registered in GlobalLedger.`);
    // Depending on requirements, you might want to throw an error here
  }
  console.log("Lending Pool address fetching complete.\n");

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
  const setOraclePriceUSDTTx = await deployerWallet.sendTransaction({
    to: deployOracle,
    data: setUSDTPrice,
  });

  await setOraclePriceUSDTTx.wait();
  console.log(
    `Oracle price set for USDT at tx hash ${setOraclePriceUSDTTx.hash}`,
  );

  // Set the price for ETH
  const setOraclePriceETHTx = await deployerWallet.sendTransaction({
    to: deployOracle,
    data: setETHPrice,
  });

  await setOraclePriceETHTx.wait();
  console.log(`Oracle price set for ETH at tx hash ${setOraclePriceETHTx.hash}`);

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

  // Get GlobalLedger contract instance for reading state later
  const globalLedgerContract = getContract({
    client,
    abi: GlobalLedger.abi,
    address: deployGlobalLedger,
  });

  // Perform a deposit of USDT by account1 into the LendingPool on shard 0
  const depositAmountUSDT = 12n;
  const depositUSDT = {
    id: process.env.USDT as `0x${string}`,
    amount: depositAmountUSDT,
  };

  console.log(`Account 1 depositing ${depositAmountUSDT} USDT via Pool on Shard ${lendingPools[0].shardId}...`);
  const depositUSDTResponseTx = await account1.sendTransaction({
    to: lendingPools[0].address,
    functionName: "deposit",
    abi: LendingPool.abi as Abi,
    tokens: [depositUSDT],
    feeCredit: convertEthToWei(0.001),
  });

  await depositUSDTResponseTx.wait();
  console.log(
    `Account 1 deposit initiated at tx hash ${depositUSDTResponseTx.hash}`
  );

  // Perform a deposit of ETH by account2 into the LendingPool on shard 2
  const depositAmountETH = 5n;
  const depositETH = {
    id: process.env.ETH as `0x${string}`,
    amount: depositAmountETH,
  };

  console.log(`Account 2 depositing ${depositAmountETH} ETH via Pool on Shard ${lendingPools[2].shardId}...`);
  const depositETHResponseTx = await account2.sendTransaction({
    to: lendingPools[2].address,
    functionName: "deposit",
    abi: LendingPool.abi as Abi,
    tokens: [depositETH],
    feeCredit: convertEthToWei(0.001),
  });

  await depositETHResponseTx.wait();
  console.log(
    `Account 2 deposit initiated at tx hash ${depositETHResponseTx.hash}`
  );

  // --- Add a delay or wait mechanism if necessary for async calls to process ---
  console.log("Waiting a few seconds for deposits to process asynchronously...");
  await new Promise(resolve => setTimeout(resolve, 5000)); // 5 second wait

  // Retrieve the collateral balances from GlobalLedger
  console.log("Checking collateral balances in GlobalLedger...");
  const account1CollateralUSDT = await globalLedgerContract.read.getCollateralBalance([
    account1.address,
    process.env.USDT as `0x${string}`,
  ]) as bigint;
  const account2CollateralETH = await globalLedgerContract.read.getCollateralBalance([
    account2.address,
    process.env.ETH as `0x${string}`,
  ]) as bigint;

  console.log(`Account 1 collateral balance (USDT) in GlobalLedger: ${account1CollateralUSDT}`);
  console.log(`Account 2 collateral balance (ETH) in GlobalLedger: ${account2CollateralETH}`);
  // Check if balances match deposited amounts
  if (account1CollateralUSDT !== depositAmountUSDT) {
    console.error(`ERROR: Account 1 USDT collateral (${account1CollateralUSDT}) does not match deposit (${depositAmountUSDT})!`);
  }
  if (account2CollateralETH !== depositAmountETH) {
    console.error(`ERROR: Account 2 ETH collateral (${account2CollateralETH}) does not match deposit (${depositAmountETH})!`);
  }

  // Perform a borrow operation by account1 for 5 ETH using USDT as collateral
  const borrowAmountETH = 5n;
  const borrowETHData = encodeFunctionData({
    abi: LendingPool.abi as Abi,
    functionName: "borrow",
    args: [borrowAmountETH, process.env.ETH],
  });

  console.log(`Account 1 borrowing ${borrowAmountETH} ETH via Pool on Shard ${lendingPools[0].shardId}...`);
  const account1TokensRecordBeforeBorrow = await client.getTokens(account1.address, "latest");
  const account1TokensBeforeBorrow = tokensRecordToArray(account1TokensRecordBeforeBorrow);
  const globalLedgerTokensRecordBeforeBorrow = await client.getTokens(deployGlobalLedger, "latest");
  const globalLedgerTokensBeforeBorrow = tokensRecordToArray(globalLedgerTokensRecordBeforeBorrow);
  console.log("Account 1 Tokens BEFORE Borrow:", account1TokensBeforeBorrow);
  console.log("GlobalLedger Tokens BEFORE Borrow:", globalLedgerTokensBeforeBorrow);

  const borrowETHResponseTx = await account1.sendTransaction({
    to: lendingPools[0].address,
    data: borrowETHData,
    feeCredit: convertEthToWei(0.001),
  });

  await borrowETHResponseTx.wait();
  console.log(
    `Account 1 borrow initiated at tx hash ${borrowETHResponseTx.hash}`
  );

  // --- Add delay for borrow processing ---
  console.log("Waiting a few seconds for borrow to process asynchronously...");
  await new Promise(resolve => setTimeout(resolve, 8000)); // 8 second wait

  // Check balances and state after borrow
  console.log("Checking state after borrow...");
  const account1TokensRecordAfterBorrow = await client.getTokens(account1.address, "latest");
  const account1TokensAfterBorrow = tokensRecordToArray(account1TokensRecordAfterBorrow);
  const globalLedgerTokensRecordAfterBorrow = await client.getTokens(deployGlobalLedger, "latest");
  const globalLedgerTokensAfterBorrow = tokensRecordToArray(globalLedgerTokensRecordAfterBorrow);
  const account1LoanDetails = await globalLedgerContract.read.getLoanDetails([account1.address]) as readonly [bigint, `0x${string}`];

  console.log("Account 1 Tokens AFTER Borrow:", account1TokensAfterBorrow);
  console.log("GlobalLedger Tokens AFTER Borrow:", globalLedgerTokensAfterBorrow);
  console.log("Account 1 Loan Details in GlobalLedger:", account1LoanDetails);

  // Validate changes (simple checks, more robust checks might be needed)
  const ethTokenInfoBefore = account1TokensBeforeBorrow.find((t: TokenInfo) => t.id === process.env.ETH as `0x${string}`);
  const ethAmountBefore = ethTokenInfoBefore ? ethTokenInfoBefore.amount : 0n;
  const ethTokenInfoAfter = account1TokensAfterBorrow.find((t: TokenInfo) => t.id === process.env.ETH as `0x${string}`);
  const ethAmountAfter = ethTokenInfoAfter ? ethTokenInfoAfter.amount : 0n;

  if (ethAmountAfter !== ethAmountBefore + borrowAmountETH) {
    console.error(`ERROR: Account 1 ETH balance did not increase correctly! Expected ${ethAmountBefore + borrowAmountETH}, got ${ethAmountAfter}`);
  }
  if (account1LoanDetails[0] !== borrowAmountETH || account1LoanDetails[1] !== process.env.ETH) {
    console.error(`ERROR: Loan details incorrect in GlobalLedger! Expected ${borrowAmountETH} ETH, got ${account1LoanDetails[0]} ${account1LoanDetails[1]}`);
  }

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
  const repayAmountETH = 6n;
  const repayETH = [
    {
      id: process.env.ETH as `0x${string}`,
      amount: repayAmountETH,
    },
  ];

  const repayETHData = encodeFunctionData({
    abi: LendingPool.abi as Abi,
    functionName: "repayLoan",
    args: [],
  });

  console.log(`Account 1 repaying ${repayAmountETH} ETH via Pool on Shard ${lendingPools[0].shardId}...`);
  const account1TokensRecordBeforeRepay = await client.getTokens(account1.address, "latest");
  const account1TokensBeforeRepay = tokensRecordToArray(account1TokensRecordBeforeRepay);
  const account1CollateralBeforeRepay = await globalLedgerContract.read.getCollateralBalance([account1.address, process.env.USDT as `0x${string}`]) as bigint;
  console.log("Account 1 Tokens BEFORE Repay:", account1TokensBeforeRepay);
  console.log("Account 1 USDT Collateral BEFORE Repay:", account1CollateralBeforeRepay);

  const repayETHResponseTx = await account1.sendTransaction({
    to: lendingPools[0].address,
    data: repayETHData,
    tokens: repayETH,
    feeCredit: convertEthToWei(0.001),
  });

  await repayETHResponseTx.wait();
  console.log(
    `Account 1 repay initiated at tx hash ${repayETHResponseTx.hash}`
  );

  // --- Add delay for repay processing ---
  console.log("Waiting a few seconds for repayment and collateral release to process asynchronously...");
  await new Promise(resolve => setTimeout(resolve, 10000)); // 10 second wait

  // Check balances and state after repay
  console.log("Checking state after repayment...");
  const account1TokensRecordAfterRepay = await client.getTokens(account1.address, "latest");
  const account1TokensAfterRepay = tokensRecordToArray(account1TokensRecordAfterRepay);
  const account1LoanDetailsAfterRepay = await globalLedgerContract.read.getLoanDetails([account1.address]) as readonly [bigint, `0x${string}`];
  const account1CollateralAfterRepay = await globalLedgerContract.read.getCollateralBalance([account1.address, process.env.USDT as `0x${string}`]) as bigint;

  console.log("Account 1 Tokens AFTER Repay:", account1TokensAfterRepay);
  console.log("Account 1 Loan Details AFTER Repay:", account1LoanDetailsAfterRepay);
  console.log("Account 1 USDT Collateral AFTER Repay:", account1CollateralAfterRepay);

  // Validate changes
  const ethTokenInfoBeforeRepay = account1TokensBeforeRepay.find((t: TokenInfo) => t.id === process.env.ETH as `0x${string}`);
  const ethAmountBeforeRepay = ethTokenInfoBeforeRepay ? ethTokenInfoBeforeRepay.amount : 0n;
  const ethTokenInfoAfterRepay = account1TokensAfterRepay.find((t: TokenInfo) => t.id === process.env.ETH as `0x${string}`);
  const ethAmountAfterRepay = ethTokenInfoAfterRepay ? ethTokenInfoAfterRepay.amount : 0n;

  const usdtTokenInfoBeforeRepay = account1TokensBeforeRepay.find((t: TokenInfo) => t.id === process.env.USDT as `0x${string}`);
  const usdtAmountBeforeRepay = usdtTokenInfoBeforeRepay ? usdtTokenInfoBeforeRepay.amount : 0n;
  const usdtTokenInfoAfterRepay = account1TokensAfterRepay.find((t: TokenInfo) => t.id === process.env.USDT as `0x${string}`);
  const usdtAmountAfterRepay = usdtTokenInfoAfterRepay ? usdtTokenInfoAfterRepay.amount : 0n;

  // Check ETH balance decreased by sent amount
  if (ethAmountAfterRepay !== ethAmountBeforeRepay - repayAmountETH) {
    console.error(`ERROR: Account 1 ETH balance did not decrease correctly after repay! Expected ${ethAmountBeforeRepay - repayAmountETH}, got ${ethAmountAfterRepay}`);
  }
  // Check loan is cleared
  if (account1LoanDetailsAfterRepay[0] !== 0n) {
    console.error(`ERROR: Loan details not cleared in GlobalLedger! Amount: ${account1LoanDetailsAfterRepay[0]}`);
  }
  // Check collateral is released (balance should be 0 in GL, and returned to user's wallet)
  if (account1CollateralAfterRepay !== 0n) {
    console.error(`ERROR: Collateral balance not cleared in GlobalLedger! Amount: ${account1CollateralAfterRepay}`);
  }
  if (usdtAmountAfterRepay !== usdtAmountBeforeRepay + account1CollateralBeforeRepay) {
    console.error(`ERROR: Account 1 USDT balance did not increase correctly after collateral release! Expected ${usdtAmountBeforeRepay + account1CollateralBeforeRepay}, got ${usdtAmountAfterRepay}`);
  }

  console.log("--- End-to-End Test Complete ---");
});
