// import libraries, abis
// Purpose: This file contains the e2e tests for the interaction page.
// Deploy all the contracts -> Cm -> IM -> lending pool on all shards : done
// Create 2 new smart accounts : Done
// Set Oracle prices  : Done
// Get oracle prices and cross check : Done
// Top up both of them by 5000 USDT and 5 ETH : Done
// Deposit 3200 USDT from account 1 and 1 ETH  from account 2 to the lending pool : Done
// Borrow 1 ETH from account 1
// Should be able to borrow 1 ETH from account 1
// Try to borrow more and fail
// Repay 1 ETH from account 1
// Should be able to repay 1 ETH from account 1
// End
import {
    PublicClient,
    HttpTransport,
    FaucetClient,
    generateSmartAccount,
    waitTillCompleted,
    getContract,
    CometaService,
    convertEthToWei
} from "@nilfoundation/niljs"

import { decodeFunctionResult, encodeFunctionData, type Abi, parseEther } from 'viem';

import { expect } from "chai";
import { task } from "hardhat/config";
import * as InterestManager from "../artifacts/contracts/InterestManager.sol/InterestManager.json";
import * as LendingPool from "../artifacts/contracts/LendingPool.sol/LendingPool.json";
import * as GlobalLedger from "../artifacts/contracts/CollateralManager.sol/GlobalLedger.json";
import * as Oracle from "../artifacts/contracts/Oracle.sol/Oracle.json";
import * as dotenv from 'dotenv';
dotenv.config();

task("e2eInteraction", "End to end test for the interaction page")
    .setAction(async (args, hre) => {

    const client = new PublicClient({
        transport: new HttpTransport({
            endpoint: process.env.NIL_RPC_ENDPOINT as string,
        }),
    }
    );

    const faucet = new FaucetClient({
        transport: new HttpTransport({
            endpoint: process.env.NIL_RPC_ENDPOINT as string,
        }),
    });
    console.log("Faucet client created");
    console.log("Deploying Wallet");
    const deployerWallet = await generateSmartAccount({
        shardId : 1,
        rpcEndpoint : process.env.NIL_RPC_ENDPOINT as string,
        faucetEndpoint : process.env.NIL_RPC_ENDPOINT as string,
    });

    console.log(`Deployer smart account generated at ${deployerWallet.address}`);

    const topUpSmartAccount = await faucet.topUpAndWaitUntilCompletion({
        smartAccountAddress: deployerWallet.address,
        faucetAddress: process.env.USDT as `0x${string}`,
        amount: 3000,
    },
    client
    );

    console.log(`Deployer smart account ${deployerWallet.address} has been topped up with 3000 USDT at tx hash ${topUpSmartAccount}`);

    const {address : deployInterestManager, hash : deployInterestManagerHash} = await deployerWallet.deployContract({
        shardId : 2,
        args : [],
        bytecode : InterestManager.bytecode as `0x${string}`,
        abi : InterestManager.abi as Abi,
        salt: BigInt(Math.floor(Math.random() * 10000)),
    });

    await waitTillCompleted(client, deployInterestManagerHash);
    console.log(`Interest Manager deployed at ${deployInterestManager} with hash ${deployInterestManagerHash} on shard 2`);
    // ;
    const {address : deployGlobalLedger, hash : deployGlobalLedgerHash} = await deployerWallet.deployContract({
        shardId : 3,
        args : [],
        bytecode : GlobalLedger.bytecode as `0x${string}`,
        abi : GlobalLedger.abi as Abi,
        salt: BigInt(Math.floor(Math.random() * 10000)),
        
    });
    
    await waitTillCompleted(client, deployGlobalLedgerHash);
    console.log(`Global Ledger deployed at ${deployGlobalLedger} with hash ${deployGlobalLedgerHash} on shard 3`);

    const {address : deployOracle, hash : deployOracleHash} = await deployerWallet.deployContract({
        shardId : 4,
        args : [],
        bytecode : Oracle.bytecode as `0x${string}`,
        abi : Oracle.abi as Abi,
        salt: BigInt(Math.floor(Math.random() * 10000)),
    });

    await waitTillCompleted(client, deployOracleHash);
    console.log(`Oracle deployed at ${deployOracle} with hash ${deployOracleHash} on shard 4`);

    const {address : deployLendingPool, hash : deployLendingPoolHash} = await deployerWallet.deployContract({
        shardId : 1,
        args : [deployGlobalLedger, deployInterestManager, deployOracle, process.env.USDT, process.env.ETH],
        bytecode : LendingPool.bytecode as `0x${string}`,
        abi : LendingPool.abi as Abi,
        salt: BigInt(Math.floor(Math.random() * 10000)),
    });

    
    await waitTillCompleted(client, deployLendingPoolHash);
    console.log(`Lending Pool deployed at ${deployLendingPool} with hash ${deployLendingPoolHash} on shard 1`);

    const account1 = await generateSmartAccount({
        shardId : 1,
        rpcEndpoint : process.env.NIL_RPC_ENDPOINT as string,
        faucetEndpoint : process.env.NIL_RPC_ENDPOINT as string,
    });

    console.log(`Account 1 generated at ${account1.address}`);

    const account2 = await generateSmartAccount({
        shardId : 3,
        rpcEndpoint : process.env.NIL_RPC_ENDPOINT as string,
        faucetEndpoint : process.env.NIL_RPC_ENDPOINT as string,
    });

    console.log(`Account 2 generated at ${account2.address}`);

    const topUpAccount1 = await faucet.topUpAndWaitUntilCompletion({
        smartAccountAddress: account1.address,
        faucetAddress: process.env.USDT as `0x${string}`,
        amount: 5000,
    },
    client
    );

    const topUpAccount1WithETH = await faucet.topUpAndWaitUntilCompletion({
        smartAccountAddress: account1.address,
        faucetAddress: process.env.ETH as `0x${string}`,
        amount: 10,
    },
    client
    );

    console.log(`Account 1 topped up with 5000 USDT at tx hash ${topUpAccount1}`);
    console.log(`Account 1 topped up with 10 ETH at tx hash ${topUpAccount1WithETH}`);

    const topUpAccount2 = await faucet.topUpAndWaitUntilCompletion({
        smartAccountAddress: account2.address,
        faucetAddress: process.env.ETH as `0x${string}`,
        amount: 5,
    },
    client
    );

    console.log(`Account 2 topped up with 5 ETH at tx hash ${topUpAccount2}`);

    console.log("Tokens in account 1:", await client.getTokens(account1.address, "latest"));
    console.log("Tokens in account 2:", await client.getTokens(account2.address, "latest")); 

    const setUSDTPrice = encodeFunctionData({
        abi : Oracle.abi as Abi,
        functionName : "setPrice",
        args: [process.env.USDT, 1]
    });

    const setETHPrice = encodeFunctionData({
        abi : Oracle.abi as Abi,
        functionName : "setPrice",
        args: [process.env.ETH, 3000]
    });

    const setOraclePrice = await deployerWallet.sendTransaction({
        to: deployOracle,
        data: setUSDTPrice,
    });

    
    await waitTillCompleted(client, setOraclePrice);
    console.log(`Oracle price set for USDT at tx hash ${setOraclePrice}`);

    const setOraclePriceETH = await deployerWallet.sendTransaction({
        to: deployOracle,
        data: setETHPrice,
    });

    ;
    await waitTillCompleted(client, setOraclePriceETH);
    console.log(`Oracle price set for ETH at tx hash ${setOraclePriceETH}`);    

    const oracleContract = getContract({
        client,
        abi : Oracle.abi,
        address : deployOracle
    });

    const usdtPriceRequest = await client.call({
        from : deployOracle,
        to : deployOracle,
        data : encodeFunctionData({
            abi : Oracle.abi as Abi,
            functionName : "getPrice",
            args: [process.env.USDT]
            }),
        },
    "latest"
    );

    // const usdtPrice = await oracleContract.read.getPrice(["0x0001111111111111111111111111111111111113"]);
    const ethPrice = await oracleContract.read.getPrice(["0x0001111111111111111111111111111111111112"]);

    console.log("TEST RESPONSE:", usdtPriceRequest.data);
    const usdtPrice = decodeFunctionResult({
        abi : Oracle.abi as Abi,
        functionName : "getPrice",
        data : usdtPriceRequest.data
    });

    console.log(`USDT price is ${usdtPrice}`);
    console.log(`ETH price is ${ethPrice}`);

    expect(usdtPrice).to.equal(1);
    expect(ethPrice).to.equal(3000);

    const depositUSDT = {
        id : process.env.USDT as `0x${string}`,
        amount : 3600n
    }

    const depositUSDTResponse = await account1.sendTransaction({
        to : deployLendingPool,
        functionName : "deposit",
        abi : LendingPool.abi as Abi,
        tokens : [depositUSDT],
        feeCredit : 5_000_000n
    });

    await waitTillCompleted(client, depositUSDTResponse);
    console.log(`Account 1 deposited 3600 USDT at tx hash ${depositUSDTResponse}`);  

    const depositETH = {
        id : process.env.ETH as `0x${string}`,
        amount : 1n
    }

    const depositETHResponse = await account2.sendTransaction({
            to: deployLendingPool,
            functionName : "deposit",
            abi : LendingPool.abi as Abi,
            tokens: [depositETH],
            feeCredit : 5_000_000n
        });
    
        await waitTillCompleted(client, depositETHResponse);
        console.log(`Account 2 deposited 1 ETH at tx hash ${depositETHResponse}`);
    

    const globalLedgerContract = getContract({
        client,
        abi : GlobalLedger.abi,
        address : deployGlobalLedger
    });

    const account1Balance = await globalLedgerContract.read.getDeposit([account1.address, process.env.USDT]);
    const account2Balance = await globalLedgerContract.read.getDeposit([account2.address, process.env.ETH]);

    console.log(`Account 1 balance in global ledger is ${account1Balance}`);
    console.log(`Account 2 balance in global ledger is ${account2Balance}`);

    const borrowETH = encodeFunctionData({
        abi : LendingPool.abi as Abi,
        functionName : "borrow",
        args: [1, process.env.ETH]
    });

    const account1BalanceBeforeBorrow = await client.getTokens(account1.address, "latest");
    console.log("Account 1 balance before borrow:", account1BalanceBeforeBorrow);

    const borrowETHResponse = await account1.sendTransaction({
        to: deployLendingPool,
        data: borrowETH,
        feeCredit : 14_000_000n
    });

    await waitTillCompleted(client, borrowETHResponse);
    console.log(`Account 1 borrowed 1 ETH at tx hash ${borrowETHResponse}`);

    const account1BalanceAfterBorrow = await client.getTokens(account1.address, "latest");
    console.log("Account 1 balance after borrow:", account1BalanceAfterBorrow);

    const repayETH = [{
        id : process.env.ETH as `0x${string}`,
        amount : 2n
    }]

    const repayETHData = encodeFunctionData({
        abi : LendingPool.abi as Abi,
        functionName : "repayLoan",
        args: []
    });

    const repayETHResponse = await account1.sendTransaction({
        to: deployLendingPool,
        data: repayETHData,
        tokens: repayETH,
        feeCredit : 14_000_000n
    });

    await waitTillCompleted(client, repayETHResponse);
    console.log(`Account 1 repaid 1 ETH at tx hash ${repayETHResponse}`);
    const account1BalanceAfterRepay = await client.getTokens(account1.address, "latest");
    console.log("Account 1 balance after repay:", account1BalanceAfterRepay);
});