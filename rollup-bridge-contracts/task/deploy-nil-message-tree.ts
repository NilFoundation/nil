import type { Abi } from "abitype";
import { task } from "hardhat/config";
import {
    FaucetClient,
    HttpTransport,
    LocalECDSAKeySigner,
    PublicClient,
    SmartAccountV1,
    convertEthToWei,
    Transaction,
    generateRandomPrivateKey,
    waitTillCompleted,
    getContract,
    ProcessedReceipt,
} from "@nilfoundation/niljs";
import { loadNilSmartAccount } from "./nil-smart-account";
import { L2NetworkConfig, loadNilNetworkConfig, saveNilNetworkConfig } from "../deploy/config/config-helper";
import { getCheckSummedAddress } from "../scripts/utils/validate-config";
import { decodeFunctionResult, encodeFunctionData } from "viem";

// npx hardhat deploy-nil-message-tree --networkname local
task("deploy-nil-message-tree", "Deploys NilMessageTree contract on Nil Chain")
    .addParam("networkname", "The network to use") // Mandatory parameter
    .setAction(async (taskArgs) => {

        // Dynamically load artifacts
        const NilMessageTreeJson = await import("../artifacts/contracts/common/NilMessageTree.sol/NilMessageTree.json");

        if (!NilMessageTreeJson || !NilMessageTreeJson.default || !NilMessageTreeJson.default.abi || !NilMessageTreeJson.default.bytecode) {
            throw Error(`Invalid NilMessageTree ABI`);
        }

        const networkName = taskArgs.networkname;
        console.log(`Running task on network: ${networkName}`);

        const deployerAccount = await loadNilSmartAccount();

        if (!deployerAccount) {
            throw Error(`Invalid Deployer SmartAccount`);
        }

        const balance = await deployerAccount.getBalance();

        if (!(balance > BigInt(0))) {
            throw Error(`Insufficient or Zero balance for smart-account: ${deployerAccount.address}`);
        }

        const { tx: nilMessageTreeDeployTxn, address: nilMessageTreeAddress } = await deployerAccount.deployContract({
            shardId: 1,
            bytecode: NilMessageTreeJson.default.bytecode as `0x${string}`,
            abi: NilMessageTreeJson.default.abi as Abi,
            args: [deployerAccount.address],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        console.log(`address from deployment is: ${nilMessageTreeAddress}`);
        await waitTillCompleted(deployerAccount.client, nilMessageTreeDeployTxn.hash, {
            waitTillMainShard: true
        });
        console.log("✅ Logic Contract deployed with transactionHash:", nilMessageTreeDeployTxn.hash);

        if (!nilMessageTreeDeployTxn.hash) {
            throw Error(`Invalid transaction output from deployContract call for NilMessageTree Contract`);
        }

        if (!nilMessageTreeAddress) {
            throw Error(`Invalid address output from deployContract call for NilMessageTree Contract`);
        }

        console.log(`NilMessageTree contract deployed at address: ${nilMessageTreeAddress} and with transactionHash: ${nilMessageTreeDeployTxn.hash}`);

        // save the nilMessageTree Address in the json config for l2
        const config: L2NetworkConfig = loadNilNetworkConfig(networkName);

        config.nilMessageTreeConfig.nilMessageTreeContracts.nilMessageTreeImplementationAddress = getCheckSummedAddress(nilMessageTreeAddress);

        const nilMessageTreeInstance = getContract({
            client: deployerAccount.client,
            abi: NilMessageTreeJson.default.abi as Abi,
            address: config.nilMessageTreeConfig.nilMessageTreeContracts.nilMessageTreeImplementationAddress as `0x${string}`
        });


        const ownerFromDeployedContract = await nilMessageTreeInstance.read.owner([]);
        if (ownerFromDeployedContract != config.l2CommonConfig.owner) {
            throw Error(`OwnerAddress in ETHBridgeContract: ${ownerFromDeployedContract} is incorrect, correct owner as per config: ${config.l2CommonConfig.owner}`);
        }

        const initialiseMessageTreeData = encodeFunctionData({
            abi: NilMessageTreeJson.default.abi as Abi,
            functionName: "initialize",
            args: [],
        });

        const initialiseMessageTreeResponse = await deployerAccount.sendTransaction({
            to: config.nilMessageTreeConfig.nilMessageTreeContracts.nilMessageTreeImplementationAddress as `0x${string}`,
            data: initialiseMessageTreeData,
            feeCredit: convertEthToWei(0.001),
        });

        const initializeMessageTreeReceipts: ProcessedReceipt[] = await initialiseMessageTreeResponse.wait();

        // check the first element in the ProcessedReceipt and verify if it is successful
        if (!initializeMessageTreeReceipts[0].success) {
            throw Error(`Failed to initialize the NilMessageTree which was deployed at address: ${config.nilMessageTreeConfig.nilMessageTreeContracts.nilMessageTreeImplementationAddress}`);
        }

        const isMerkleTreeInitializedResponse = await nilMessageTreeInstance.read.isMerkleTreeInitialized([]);

        if (!isMerkleTreeInitializedResponse) {
            throw Error(`Failed to assert the initialisation transaction on NilMessageTree at address: ${config.nilMessageTreeConfig.nilMessageTreeContracts.nilMessageTreeImplementationAddress}`);
        }

        console.log(`successfully initialized the NilMessageTree at address: ${config.nilMessageTreeConfig.nilMessageTreeContracts.nilMessageTreeImplementationAddress}`);

        // Save the updated config
        saveNilNetworkConfig(networkName, config);
    });