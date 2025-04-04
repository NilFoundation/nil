import {
    FaucetClient,
    HttpTransport,
    PublicClient,
    convertEthToWei,
    generateSmartAccount,
    waitTillCompleted,
    type Token,
} from "@nilfoundation/niljs";

import { type Abi, encodeFunctionData } from "viem";

import * as dotenv from "dotenv";
import { task } from "hardhat/config";
dotenv.config();

task(
    "deploy-test-tokensplitter",
    "Deploys TokenSplitter, sends tokens to it, splits them, and verifies balances"
).setAction(async () => {
    console.log("🚀 Starting TokenSplitter Deployment and Test Script");

    // --- Configuration ---
    const deployerShard = 1;
    const recipientShards = [2, 3, 4];
    const tokenToSplitId = process.env.USDT as `0x${string}`;
    if (!tokenToSplitId) {
        throw new Error("❌ Missing USDT environment variable");
    }
    const amountsToSplit = [10n, 20n, 5n];
    const totalAmountToSplit = amountsToSplit.reduce((sum, amount) => sum + amount, 0n);
    const fundingAmount = totalAmountToSplit + 5n;
    const feeCredit = convertEthToWei(0.005);

    // --- Import Contract Artifact ---
    const TokenSplitter = require("../artifacts/contracts/tokenSplitter.sol/TokenSplitter.json");
    if (!TokenSplitter.abi || !TokenSplitter.bytecode) {
        throw new Error("❌ TokenSplitter ABI or bytecode not found. Compile contracts first.");
    }

    // --- Initialize Clients ---
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

    // --- Generate Accounts ---
    console.log("👤 Generating Deployer and Recipient Accounts...");
    const deployerWallet = await generateSmartAccount({
        shardId: deployerShard,
        rpcEndpoint: process.env.NIL_RPC_ENDPOINT as string,
        faucetEndpoint: process.env.NIL_RPC_ENDPOINT as string,
    });
    console.log(`Deployer account generated: ${deployerWallet.address} (Shard ${deployerShard})`);

    const recipients: Awaited<ReturnType<typeof generateSmartAccount>>[] = [];
    for (let i = 0; i < recipientShards.length; i++) {
        const recipient = await generateSmartAccount({
            shardId: recipientShards[i],
            rpcEndpoint: process.env.NIL_RPC_ENDPOINT as string,
            faucetEndpoint: process.env.NIL_RPC_ENDPOINT as string,
        });
        console.log(`Recipient ${i + 1} account generated: ${recipient.address} (Shard ${recipientShards[i]})`);
        recipients.push(recipient);
    }
    const recipientAddresses = recipients.map(r => r.address);

    // --- Fund Deployer Account ---
    console.log(`💰 Funding Deployer ${deployerWallet.address} with ${fundingAmount} ${tokenToSplitId}...`);
    try {
        await faucet.topUpAndWaitUntilCompletion(
            {
                smartAccountAddress: deployerWallet.address,
                faucetAddress: tokenToSplitId,
                amount: fundingAmount,
            },
            client
        );
        console.log(`✅ Deployer funded with ${tokenToSplitId}.`);
    } catch (error) {
        console.error(`❌ Failed to fund Deployer with ${tokenToSplitId}. Make sure the faucet address is correct and has funds.`, error);
        console.log(`💰 Attempting to fund Deployer ${deployerWallet.address} with NIL for gas...`);
        try {
            await faucet.topUpAndWaitUntilCompletion(
                {
                    smartAccountAddress: deployerWallet.address,
                    faucetAddress: process.env.NIL as `0x${string}`,
                    amount: convertEthToWei(0.1),
                },
                client
            );
            console.log("✅ Deployer funded with NIL.");
        } catch (nilError) {
            console.error("❌ Failed to fund Deployer with NIL as well.", nilError);
            throw new Error("Funding failed for both token and NIL.");
        }
        throw new Error(`Funding with ${tokenToSplitId} failed, but NIL funding succeeded. Cannot proceed without the token to split.`);
    }


    // --- Deploy TokenSplitter Contract ---
    console.log(`🏗 Deploying TokenSplitter contract from ${deployerWallet.address}...`);
    const { address: tokenSplitterAddress, hash: deployHash } = await deployerWallet.deployContract({
        shardId: deployerShard,
        abi: TokenSplitter.abi as Abi,
        bytecode: TokenSplitter.bytecode as `0x${string}`,
        salt: BigInt(Math.floor(Math.random() * 100000)),
        feeCredit: feeCredit,
    });
    await waitTillCompleted(client, deployHash);
    console.log(`✅ TokenSplitter deployed at: ${tokenSplitterAddress} (Tx: ${deployHash})`);

    // --- Transfer Tokens to TokenSplitter ---
    console.log(`💸 Transferring ${totalAmountToSplit} ${tokenToSplitId} from Deployer to TokenSplitter (${tokenSplitterAddress})...`);
    const transferTokens: Token[] = [
        {
            id: tokenToSplitId,
            amount: totalAmountToSplit,
        },
    ];
    const transferTxData = {
        to: tokenSplitterAddress,
        tokens: transferTokens,
        feeCredit: feeCredit,
        // No data needed for a simple token transfer via Nil.sendTransaction
    };

    const transferHash = await deployerWallet.sendTransaction(transferTxData);
    await waitTillCompleted(client, transferHash);
    console.log(`✅ Tokens transferred to TokenSplitter (Tx: ${transferHash})`);

    // --- Verify TokenSplitter Balance (Optional but Recommended) ---
    await new Promise(res => setTimeout(res, 5000)); // Wait a bit for state sync
    console.log(`🔍 Verifying TokenSplitter balance...`);
    const splitterTokenRecord: Record<string, bigint> = await client.getTokens(tokenSplitterAddress, 'latest');
    const splitterBalance: bigint = splitterTokenRecord[tokenToSplitId] ?? 0n;
    console.log(`TokenSplitter ${tokenToSplitId} balance: ${splitterBalance}`);
    if (splitterBalance < totalAmountToSplit) {
        console.warn(`⚠️ TokenSplitter balance (${splitterBalance}) is less than expected (${totalAmountToSplit}). Continuing, but splitting might fail.`);
    } else {
        console.log("✅ TokenSplitter balance verified.");
    }

    // --- Get Initial Recipient Balances ---
    console.log("🔍 Getting initial recipient balances...");
    const initialRecipientBalances: (bigint | undefined)[] = [];
    for (let i = 0; i < recipients.length; i++) {
        const initialTokenRecord: Record<string, bigint> = await client.getTokens(recipients[i].address, 'latest');
        initialRecipientBalances[i] = initialTokenRecord[tokenToSplitId];
        console.log(`Recipient ${i + 1} (${recipients[i].address}) initial ${tokenToSplitId} balance: ${initialRecipientBalances[i] ?? 0n}`);
    }

    // --- Call splitTokens ---
    console.log(`⚡ Calling splitTokens function on ${tokenSplitterAddress}...`);
    const splitArgs = [
        tokenToSplitId,
        recipientAddresses,
        amountsToSplit,
    ];
    const splitTxData = encodeFunctionData({
        abi: TokenSplitter.abi as Abi,
        functionName: "splitTokens",
        args: splitArgs,
    });

    const splitHash = await deployerWallet.sendTransaction({
        to: tokenSplitterAddress,
        data: splitTxData,
        feeCredit: feeCredit,
    });
    await waitTillCompleted(client, splitHash);
    console.log(`✅ splitTokens function called (Tx: ${splitHash})`);

    // --- Verify Recipient Balances ---
    console.log("⏳ Waiting for asynchronous transfers to complete (approx 15-30 seconds)...");
    await new Promise(res => setTimeout(res, 30000));

    console.log("🔍 Verifying final recipient balances...");
    let success = true;
    for (let i = 0; i < recipients.length; i++) {
        const recipientAddress = recipients[i].address;
        try {
            const finalTokenRecord: Record<string, bigint> = await client.getTokens(recipientAddress, 'latest');
            const finalBalance: bigint = finalTokenRecord[tokenToSplitId] ?? 0n;
            const expectedBalance = (initialRecipientBalances[i] ?? 0n) + amountsToSplit[i];

            console.log(`Recipient ${i + 1} (${recipientAddress}) final ${tokenToSplitId} balance: ${finalBalance} (Expected: ${expectedBalance})`);

            if (finalBalance !== expectedBalance) {
                console.error(`❌ Verification Failed for Recipient ${i + 1}: Expected ${expectedBalance}, got ${finalBalance}`);
                success = false;
            } else {
                console.log(`✅ Verification Success for Recipient ${i + 1}`);
            }
        } catch (error) {
            console.error(`❌ Error fetching balance for Recipient ${i + 1} (${recipientAddress}):`, error);
            success = false;
        }
    }

    if (success) {
        console.log("🎉 Token splitting test completed successfully!");
    } else {
        console.error("❌ Token splitting test failed due to balance mismatches.");
        throw new Error("Token splitting verification failed.");
    }

});
