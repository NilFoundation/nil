import { expect } from "chai";
import "@nomicfoundation/hardhat-ethers";
import {
    convertEthToWei,
    FaucetClient,
    HttpTransport,
    ProcessedReceipt,
    PublicClient,
    SmartAccountV1,
    waitTillCompleted,
    getContract
} from "@nilfoundation/niljs";
import "dotenv/config";
import type { Abi } from "abitype";
import { getCheckSummedAddress } from "../../scripts/utils/validate-config";
import { decodeFunctionResult, encodeFunctionData } from "viem";
import { loadNilSmartAccount } from "../../task/nil-smart-account";

const l1EthBridgeAddress = '0x0001e0d8f4De4E838a66963f406Fa826cCaCA322';

describe("L2BridgeMessenger Contract", () => {
    it("Should accept the (ETHDeposit) message relayed by relayer", async () => {

        const smartAccount: SmartAccountV1 | null = await loadNilSmartAccount();

        if (!smartAccount) {
            throw Error(`Invalid Deployer SmartAccount`);
        }

        const rpcEndpoint = process.env.NIL_RPC_ENDPOINT as string;

        const client = new PublicClient({
            transport: new HttpTransport({ endpoint: rpcEndpoint }),
        });
        const faucetClient = new FaucetClient({
            transport: new HttpTransport({ endpoint: rpcEndpoint }),
        });

        // ##### Fund Deployer Wallet #####

        const topUpFaucetTxnHash = await faucetClient.topUp({
            smartAccountAddress: smartAccount.address,
            amount: convertEthToWei(100),
            faucetAddress: process.env.NIL as `0x${string}`,
        });

        await waitTillCompleted(client, topUpFaucetTxnHash);

        const balance = await smartAccount.getBalance();

        if (!(balance > BigInt(0))) {
            throw Error(`Insufficient or Zero balance for smart-account: ${smartAccount.address}`);
        }

        // ##### NilMessageTree Deployment ##### 

        // Dynamically load artifacts
        const NilMessageTreeJson = await import("../../artifacts/contracts/common/NilMessageTree.sol/NilMessageTree.json");

        if (!NilMessageTreeJson || !NilMessageTreeJson.default || !NilMessageTreeJson.default.abi || !NilMessageTreeJson.default.bytecode) {
            throw Error(`Invalid NilMessageTree ABI`);
        }

        const { tx: nilMessageTreeDeployTxn, address: nilMessageTreeAddress } = await smartAccount.deployContract({
            shardId: 1,
            bytecode: NilMessageTreeJson.default.bytecode as `0x${string}`,
            abi: NilMessageTreeJson.default.abi as Abi,
            args: [smartAccount.address],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await verifyDeploymentCompletion(nilMessageTreeDeployTxn, smartAccount.client, "NilMessageTree");

        if (!nilMessageTreeDeployTxn.hash) {
            throw Error(`Invalid transaction output from deployContract call for NilMessageTree Contract`);
        }

        if (!nilMessageTreeAddress) {
            throw Error(`Invalid address output from deployContract call for NilMessageTree Contract`);
        }

        console.log(`NilMessageTree contract deployed at address: ${nilMessageTreeAddress} and with transactionHash: ${nilMessageTreeDeployTxn.hash} `);


        // ##### L2ETHBridgeVault Deployment #####

        // Dynamically load artifacts
        const L2ETHBridgeVaultJson = await import("../../artifacts/contracts/bridge/l2/L2ETHBridgeVault.sol/L2ETHBridgeVault.json");
        const TransparentUpgradeableProxy = await import("../../artifacts/contracts/common/TransparentUpgradeableProxy.sol/MyTransparentUpgradeableProxy.json");

        if (!L2ETHBridgeVaultJson || !L2ETHBridgeVaultJson.default || !L2ETHBridgeVaultJson.default.abi || !L2ETHBridgeVaultJson.default.bytecode) {
            throw Error(`Invalid L2ETHBridgeVault ABI`);
        }

        const { tx: l2EthBridgeVaultImplementationDeploymentTx, address: l2EthBridgeVaultImplementationAddress } = await smartAccount.deployContract({
            shardId: 1,
            bytecode: L2ETHBridgeVaultJson.default.bytecode as `0x${string} `,
            abi: L2ETHBridgeVaultJson.default.abi as Abi,
            args: [],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001)
        });

        await verifyDeploymentCompletion(l2EthBridgeVaultImplementationDeploymentTx, smartAccount.client, "L2ETHBridgeVault");

        if (!l2EthBridgeVaultImplementationDeploymentTx || !l2EthBridgeVaultImplementationDeploymentTx.hash) {
            throw Error(`Invalid transaction output from deployContract call for L2ETHBridgeVault Contract`);
        }

        if (!l2EthBridgeVaultImplementationAddress) {
            throw Error(`Invalid address output from deployContract call for L2ETHBridgeVault Contract`);
        }

        const l2EthBridgeVaultInitData = encodeFunctionData({
            abi: L2ETHBridgeVaultJson.default.abi,
            functionName: "initialize",
            args: [smartAccount.address, smartAccount.address],
        });

        const { tx: l2EthBridgeVaultProxyDeploymentTx, address: l2EthBridgeVaultProxy } = await smartAccount.deployContract({
            shardId: 1,
            bytecode: TransparentUpgradeableProxy.default.bytecode as `0x${string} `,
            abi: TransparentUpgradeableProxy.default.abi as Abi,
            args: [l2EthBridgeVaultImplementationAddress, smartAccount.address, l2EthBridgeVaultInitData],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await verifyDeploymentCompletion(l2EthBridgeVaultProxyDeploymentTx, smartAccount.client, "L2ETHBridgeVaultProxy");

        const topUpFaucet = await faucetClient.topUp({
            smartAccountAddress: l2EthBridgeVaultProxy as `0x${string} `,
            amount: convertEthToWei(200),
            faucetAddress: process.env.NIL as `0x${string} `,
        });

        const fundL2ETHBridgeVaultTxnReceipts: ProcessedReceipt[] = await waitTillCompleted(smartAccount.client, topUpFaucet);

        // check the first element in the ProcessedReceipt and verify if it is successful
        if (!fundL2ETHBridgeVaultTxnReceipts[0].success) {
            throw Error(`Failed to fund L2ETHBridgeVault: ${l2EthBridgeVaultProxy} `);
        }

        const balanceAfterFunding = await smartAccount.client.getBalance(l2EthBridgeVaultProxy as `0x${string} `);

        const l2EthBridgeVaultProxyAddress = getCheckSummedAddress(l2EthBridgeVaultProxy);

        // ##### L2BridgeMessenger Deployment ##### 

        // Dynamically load artifacts
        const L2BridgeMessengerJson = await import("../../artifacts/contracts/bridge/l2/L2BridgeMessenger.sol/L2BridgeMessenger.json");

        if (!L2BridgeMessengerJson || !L2BridgeMessengerJson.default || !L2BridgeMessengerJson.default.abi || !L2BridgeMessengerJson.default.bytecode) {
            throw Error(`Invalid L2BridgeMessengerJson ABI`);
        }

        const { tx: nilMessengerImplementationDeploymentTx, address: nilMessengerImplementationAddress } = await smartAccount.deployContract({
            shardId: 1,
            bytecode: L2BridgeMessengerJson.default.bytecode as `0x${string} `,
            abi: L2BridgeMessengerJson.default.abi as Abi,
            args: [],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await verifyDeploymentCompletion(nilMessengerImplementationDeploymentTx, smartAccount.client, "L2BridgeMessenger");


        if (!nilMessengerImplementationDeploymentTx || !nilMessengerImplementationDeploymentTx.hash) {
            throw Error(`Invalid transaction output from deployContract call for L2BridgeMessenger Contract`);
        }

        if (!nilMessengerImplementationAddress) {
            throw Error(`Invalid address output from deployContract call for L2BridgeMessenger Contract`);
        }

        const l2BridgeMessengerImplementationAddress = getCheckSummedAddress(nilMessengerImplementationAddress);

        const l2BridgeMessengerInitData = encodeFunctionData({
            abi: L2BridgeMessengerJson.default.abi,
            functionName: "initialize",
            args: [smartAccount.address,
            smartAccount.address,
            smartAccount.address,
                nilMessageTreeAddress,
                1000000],
        });

        const { tx: l2BridgeMessengerProxyDeploymentTx, address: l2BridgeMessengerProxy } = await smartAccount.deployContract({
            shardId: 1,
            bytecode: TransparentUpgradeableProxy.default.bytecode as `0x${string} `,
            abi: TransparentUpgradeableProxy.default.abi as Abi,
            args: [l2BridgeMessengerImplementationAddress, smartAccount.address, l2BridgeMessengerInitData],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await verifyDeploymentCompletion(l2BridgeMessengerProxyDeploymentTx, smartAccount.client, "L2BridgeMessengerProxy");

        const l2BridgeMessengerProxyAddress = getCheckSummedAddress(l2BridgeMessengerProxy);

        let l2BridgeMessengerProxyInst;

        try {
            // verify if the bridges are really authorised
            l2BridgeMessengerProxyInst = getContract({
                client: smartAccount.client,
                abi: L2BridgeMessengerJson.default.abi as Abi,
                address: l2BridgeMessengerProxyAddress as `0x${string} `
            });

        } catch (err) {
            console.error(`Error caught while loading an instance of L2BridgeMessenger: ${l2BridgeMessengerProxyAddress} `);
        }

        // Dynamically load artifacts
        const L2ETHBridgeJson = await import("../../artifacts/contracts/bridge/l2/L2ETHBridge.sol/L2ETHBridge.json");

        if (!L2ETHBridgeJson || !L2ETHBridgeJson.default || !L2ETHBridgeJson.default.abi || !L2ETHBridgeJson.default.bytecode) {
            throw Error(`Invalid L2ETHBridge ABI`);
        }

        // #####  l2ETHBridge Deployment ##### 

        const { tx: l2EthBridgeImplementationDeploymentTx, address: l2EthBridgeImplementation } = await smartAccount.deployContract({
            shardId: 1,
            bytecode: L2ETHBridgeJson.default.bytecode as `0x${string} `,
            abi: L2ETHBridgeJson.default.abi as Abi,
            args: [],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await verifyDeploymentCompletion(l2EthBridgeImplementationDeploymentTx, smartAccount.client, "L2ETHBridge");

        if (!l2EthBridgeImplementationDeploymentTx.hash) {
            throw Error(`Invalid transaction output from deployContract call for L2ETHBridge Contract`);
        }

        if (!l2EthBridgeImplementation) {
            throw Error(`Invalid address output from deployContract call for L2ETHBridge Contract`);
        }

        const l2ETHBridgeImplementationAddress = getCheckSummedAddress(l2EthBridgeImplementation);

        const l2EthBridgeInitData = encodeFunctionData({
            abi: L2ETHBridgeJson.default.abi,
            functionName: "initialize",
            args: [smartAccount.address,
            smartAccount.address,
                l2BridgeMessengerProxyAddress,
                l2EthBridgeVaultProxyAddress],
        });
        const { tx: l2EthBridgeProxyDeploymentTx, address: l2EthBridgeProxy } = await smartAccount.deployContract({
            shardId: 1,
            bytecode: TransparentUpgradeableProxy.default.bytecode as `0x${string} `,
            abi: TransparentUpgradeableProxy.default.abi as Abi,
            args: [l2ETHBridgeImplementationAddress, smartAccount.address, l2EthBridgeInitData],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await verifyDeploymentCompletion(l2EthBridgeProxyDeploymentTx, smartAccount.client, "L2ETHBridgeProxy");

        const l2ETHBridgeProxyAddress = getCheckSummedAddress(l2EthBridgeProxy);

        // Dynamically load artifacts
        const L2EnshrinedTokenBridgeJson = await import("../../artifacts/contracts/bridge/l2/L2EnshrinedTokenBridge.sol/L2EnshrinedTokenBridge.json");

        if (!L2EnshrinedTokenBridgeJson || !L2EnshrinedTokenBridgeJson.default || !L2EnshrinedTokenBridgeJson.default.abi || !L2EnshrinedTokenBridgeJson.default.bytecode) {
            throw Error(`Invalid L2EnshrinedTokenBridge ABI`);
        }

        const { tx: l2EnshrinedTokenBridgeImplDepTx, address: l2EnshrinedTokenBridgeImpl } = await smartAccount.deployContract({
            shardId: 1,
            bytecode: L2EnshrinedTokenBridgeJson.default.bytecode as `0x${string} `,
            abi: L2EnshrinedTokenBridgeJson.default.abi as Abi,
            args: [],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await verifyDeploymentCompletion(l2EnshrinedTokenBridgeImplDepTx, smartAccount.client, "L2EnshrinedTokenBridge");

        if (!l2EnshrinedTokenBridgeImplDepTx || !l2EnshrinedTokenBridgeImplDepTx.hash) {
            throw Error(`Invalid transaction output from deployContract call for L2EnshrinedTokenBridge Contract`);
        }

        if (!l2EnshrinedTokenBridgeImpl) {
            throw Error(`Invalid address output from deployContract call for L2EnshrinedTokenBridge Contract`);
        }

        const l2EnshrinedTokenBridgeImplementationAddress = getCheckSummedAddress(l2EnshrinedTokenBridgeImpl);

        const l2EnshrinedTokenBridgeInitData = encodeFunctionData({
            abi: L2EnshrinedTokenBridgeJson.default.abi,
            functionName: "initialize",
            args: [smartAccount.address,
            smartAccount.address,
                l2BridgeMessengerProxyAddress],
        });

        const { tx: l2EnshrinedTokenBridgeProxyDeploymentTx, address: l2EnshrinedTokenBridgeProxy } = await smartAccount.deployContract({
            shardId: 1,
            bytecode: TransparentUpgradeableProxy.default.bytecode as `0x${string} `,
            abi: TransparentUpgradeableProxy.default.abi as Abi,
            args: [l2EnshrinedTokenBridgeImplementationAddress, smartAccount.address, l2EnshrinedTokenBridgeInitData],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await verifyDeploymentCompletion(l2EnshrinedTokenBridgeProxyDeploymentTx, smartAccount.client, "L2EnshrinedTokenBridgeProxy");


        const l2EnshrinedTokenBridgeProxyAddress = getCheckSummedAddress(l2EnshrinedTokenBridgeProxy);

        try {
            const isL2EnshrinedTokenBridgeAuthorised = await l2BridgeMessengerProxyInst.read.isAuthorisedBridge([l2EnshrinedTokenBridgeProxyAddress]);
            if (!isL2EnshrinedTokenBridgeAuthorised) {
                console.error(`L2EnshrinedTokenBridge: ${l2EnshrinedTokenBridgeProxyAddress} is not authorised on L2BridgeMessenger: ${l2BridgeMessengerProxyAddress} `);
            }
        } catch (error) {
            console.error(`Error caught while verifying the authorisation of l2EnshrinedTokenBridge: ${l2EnshrinedTokenBridgeProxyAddress} on L2BridgeMessenger: ${l2BridgeMessengerProxyAddress} `)
        }

        const authoriseBridgesData = encodeFunctionData({
            abi: L2BridgeMessengerJson.default.abi as Abi,
            functionName: "authoriseBridges",
            args: [[l2ETHBridgeProxyAddress,
                l2EnshrinedTokenBridgeProxyAddress]
            ],
        });

        let authoriseL2BridgesTxnReceipts: ProcessedReceipt[];

        try {
            const authoriseL2BridgesResponse = await smartAccount.sendTransaction({
                to: l2BridgeMessengerProxyAddress as `0x${string} `,
                data: authoriseBridgesData,
                feeCredit: convertEthToWei(0.001),
            });

            authoriseL2BridgesTxnReceipts = await authoriseL2BridgesResponse.wait();
        } catch (err) {
            console.error(`Error caught when the bridges are being authorised on L2BridgeMessenger: ${l2BridgeMessengerProxyAddress} `, err);
        }

        const authoriseL2Bridges_outputProcessedReceipt = authoriseL2BridgesTxnReceipts?.[0]?.outputReceipts?.[0];
        if (
            !authoriseL2Bridges_outputProcessedReceipt?.success ||
            authoriseL2Bridges_outputProcessedReceipt.status === '' ||
            (typeof authoriseL2Bridges_outputProcessedReceipt.status === 'string' && authoriseL2Bridges_outputProcessedReceipt.status.toLowerCase().includes('reverted'))
        ) {
            console.error(`❌ Failed to authorise Bridges: ${[l2ETHBridgeProxyAddress,
                l2EnshrinedTokenBridgeProxyAddress]} 
                    on the L2BridgeMessenger contract: ${l2BridgeMessengerProxyAddress}`);
        } else {
            console.log(`✅ Successfully authorised Bridges: ${[l2ETHBridgeProxyAddress,
                l2EnshrinedTokenBridgeProxyAddress]} 
                    on the L2BridgeMessenger contract: ${l2BridgeMessengerProxyAddress}`);
        }

        let l2BridgeMessengerProxyInstance;

        try {
            // verify if the bridges are really authorised
            l2BridgeMessengerProxyInstance = getContract({
                client: smartAccount.client,
                abi: L2BridgeMessengerJson.default.abi as Abi,
                address: l2BridgeMessengerProxyAddress as `0x${string}`
            });

            const isL2EnshrinedTokenBridgeAuthorised = await l2BridgeMessengerProxyInstance.read.isAuthorisedBridge([l2EnshrinedTokenBridgeProxyAddress]);
            if (!isL2EnshrinedTokenBridgeAuthorised) {
                console.error(`❌ L2EnshrinedTokenBridge: ${l2EnshrinedTokenBridgeProxyAddress} is not authorised on L2BridgeMessenger: ${l2BridgeMessengerProxyAddress}`);
                expect.fail(`L2EnshrinedTokenBridge: ${l2EnshrinedTokenBridgeProxyAddress} is not authorised on L2BridgeMessenger: ${l2BridgeMessengerProxyAddress}`);
            }

            const isL2ETHBridgeAuthorised = await l2BridgeMessengerProxyInstance.read.isAuthorisedBridge([l2ETHBridgeProxyAddress]);
            if (!isL2ETHBridgeAuthorised) {
                console.error(`❌ L2ETHBridge: ${l2ETHBridgeProxyAddress} is not authorised on L2BridgeMessenger: ${l2BridgeMessengerProxyAddress}`);
                expect.fail(`L2ETHBridge: ${l2ETHBridgeProxyAddress} is not authorised on L2BridgeMessenger: ${l2BridgeMessengerProxyAddress}`);
            }

        } catch (err) {
            console.error(`❌ Error caught while getting an instance of L2BridgeMessenger: ${l2BridgeMessengerProxyAddress} `);
            expect.fail(`❌ Error caught while getting an instance of L2BridgeMessenger: ${l2BridgeMessengerProxyAddress} `);
        }
    });
});

async function verifyDeploymentCompletion(deployTxn: any, publicClient: PublicClient, contractName: string): Promise<boolean> {

    const deployTxnReceipt = await waitTillCompleted(publicClient, deployTxn.hash, {
        waitTillMainShard: true
    });

    const output = deployTxnReceipt?.[0]?.outputReceipts?.[0];
    if (
        !output?.success ||
        output.status === '' ||
        (typeof output.status === 'string' && output.status.toLowerCase().includes('reverted'))
    ) {
        console.error(`❌ Failed to deploy ${contractName} contract`);
        return false;
    } else {
        console.log(`✅ Successfully deployed ${contractName} contract`);
        return true;
    }
}