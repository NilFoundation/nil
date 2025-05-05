import type { Abi } from "abitype";
import { task } from "hardhat/config";
import {
    FaucetClient,
    HttpTransport,
    LocalECDSAKeySigner,
    PublicClient,
    SmartAccountV1,
    getContract,
    convertEthToWei,
    Transaction,
    generateRandomPrivateKey,
    waitTillCompleted,
    type ProcessedReceipt,
} from "@nilfoundation/niljs";
import { loadNilSmartAccount } from "./nil-smart-account";
import { decodeFunctionResult, encodeFunctionData } from "viem";

// npx hardhat deploy-my-logic-basic
task("deploy-my-logic-basic", "Deploys MyLogic contract on Nil Chain")
    .setAction(async (taskArgs) => {

        // Dynamically load artifacts
        const MyLogicJson = await import("../artifacts/contracts/bridge/l2/MyLogicBasic.sol/MyLogicBasic.json");
        const TransparentUpgradeableProxy = await import("../artifacts/contracts/common/TransparentUpgradeableProxy.sol/MyTransparentUpgradeableProxy.json");
        const ProxyAdmin = await import("../artifacts/node_modules/@openzeppelin/contracts/proxy/transparent/ProxyAdmin.sol/ProxyAdmin.json");

        if (!MyLogicJson || !MyLogicJson.default || !MyLogicJson.default.abi || !MyLogicJson.default.bytecode) {
            throw Error(`Invalid myLogic ABI`);
        }

        const deployerAccount = await loadNilSmartAccount();

        if (!deployerAccount) {
            throw Error(`Invalid Deployer SmartAccount`);
        }

        const balance = await deployerAccount.getBalance();

        console.log(`deployer-smart-account: ${deployerAccount.address} is on shard: ${deployerAccount.shardId} with balance: ${balance}`);

        if (!(balance > BigInt(0))) {
            throw Error(`Insufficient or Zero balance for smart-account: ${deployerAccount.address}`);
        }

        if (!MyLogicJson.default.bytecode || !/^0x[0-9a-fA-F]*$/.test(MyLogicJson.default.bytecode)) {
            throw Error(`Invalid bytecode in the ABI Json file`);
        }

        if (!MyLogicJson.default.abi) {
            throw new Error(`Invalid abi in the ABI Json file`);
        }

        const client = new PublicClient({
            transport: new HttpTransport({
                endpoint: "http://127.0.0.1:8529",
            }),
            shardId: 1
        });

        const gasPrice = await client.getGasPrice(1);

        const { tx: myLogicImplementationDeployTxn, address: myLogicImplementationAddress } = await deployerAccount.deployContract({
            shardId: 1,
            bytecode: MyLogicJson.default.bytecode as `0x${string}`,
            abi: MyLogicJson.default.abi as Abi,
            args: [],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.01),
        });

        const implementationDeploymentTxnReceipt = await myLogicImplementationDeployTxn.wait();
        if (implementationDeploymentTxnReceipt.some((receipt: ProcessedReceipt) => !receipt.success)) {
            throw new Error("implementation deployment failed");
        }

        console.log("✅ Logic Contract deployed at:", myLogicImplementationAddress);

        if (!myLogicImplementationDeployTxn.hash) {
            throw Error(`Invalid transaction output from deployContract call for myLogic Contract`);
        }

        if (!myLogicImplementationAddress) {
            throw Error(`Invalid address output from deployContract call for myLogic Contract`);
        }

        console.log(`MyLogicImplementation contract deployed at address: ${myLogicImplementationAddress} and with transactionHash: ${myLogicImplementationDeployTxn.hash}`);

        const initData = encodeFunctionData({
            abi: MyLogicJson.default.abi,
            functionName: "initialize",
            args: [deployerAccount.address, deployerAccount.address, 999],
        });

        const { tx: myLogicProxyDeployTxn, address: myLogicProxyAddress } = await deployerAccount.deployContract({
            shardId: 1,
            bytecode: TransparentUpgradeableProxy.default.bytecode as `0x${string}`,
            abi: TransparentUpgradeableProxy.default.abi as Abi,
            args: [myLogicImplementationAddress, deployerAccount.address, initData],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            //feeCredit: convertEthToWei(0.001),
        });
        const proxyDeploymentTxnReceipt = await myLogicProxyDeployTxn.wait();
        if (proxyDeploymentTxnReceipt.some((receipt: ProcessedReceipt) => !receipt.success)) {
            throw new Error("proxy deployment failed");
        }

        console.log("✅ Transparent Proxy Contract deployed at:", myLogicProxyAddress);

        console.log("Waiting 5 seconds...");
        await new Promise((res) => setTimeout(res, 10000));

        const myLogicContractInstance = getContract({
            client: deployerAccount.client,
            abi: MyLogicJson.default.abi,
            address: myLogicProxyAddress,
        });

        console.log("Properties of myLogicContractInstance:", Object.keys(myLogicContractInstance.read));

        const implementationAddress = await myLogicContractInstance.read.getImplementation([]);
        const value = await myLogicContractInstance.read.getValue([]);
        const owner = await myLogicContractInstance.read.owner([]);
        const storval = await myLogicContractInstance.read.getSimpleStorageValue([]);
        console.log(`deployerAccount address is: ${deployerAccount.address}`);
        const hasOwnerRoleBool = await myLogicContractInstance.read.hasOwnerRole([deployerAccount.address]);
        const allOwners = await myLogicContractInstance.read.getAllOwners([]);
        const allAdmins = await myLogicContractInstance.read.getAllAdmins([]);

        console.log(`implementationAddress from MyLogic is: ${implementationAddress}`);
        console.log(`value from MyLogic is: ${value}`);
        console.log(`owner from MyLogic is: ${owner}`);
        console.log(`storval: ${storval}`)
        console.log(`hasOwnerRoleBool is: ${hasOwnerRoleBool}`);
        console.log(`allOwners are: ${allOwners}`);
        console.log(`allAdmins are: ${allAdmins}`);
    });
