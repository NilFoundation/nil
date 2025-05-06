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
} from "@nilfoundation/niljs";
import { loadNilSmartAccount } from "./nil-smart-account";
import { L2NetworkConfig, loadNilNetworkConfig, saveNilNetworkConfig } from "../deploy/config/config-helper";
import { decodeFunctionResult, encodeFunctionData } from "viem";
import { getCheckSummedAddress, validateAddress } from "../scripts/utils/validate-config";
import { ZeroAddress } from "ethers";

// npx hardhat deploy-l2-eth-bridge-vault --networkname local
task("deploy-l2-eth-bridge-vault", "Deploys L2ETHBridgeVault contract on Nil Chain")
    .addParam("networkname", "The network to use") // Mandatory parameter
    .setAction(async (taskArgs) => {

        // Dynamically load artifacts
        const L2ETHBridgeVaultJson = await import("../artifacts/contracts/bridge/l2/L2ETHBridgeVault.sol/L2ETHBridgeVault.json");
        const TransparentUpgradeableProxy = await import("../artifacts/contracts/common/TransparentUpgradeableProxy.sol/MyTransparentUpgradeableProxy.json");
        const ProxyAdmin = await import("../artifacts/node_modules/@openzeppelin/contracts/proxy/transparent/ProxyAdmin.sol/ProxyAdmin.json");

        if (!L2ETHBridgeVaultJson || !L2ETHBridgeVaultJson.default || !L2ETHBridgeVaultJson.default.abi || !L2ETHBridgeVaultJson.default.bytecode) {
            throw Error(`Invalid L2ETHBridgeVault ABI`);
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

        // save the nilMessageTree Address in the json config for l2
        const l2NetworkConfig: L2NetworkConfig = loadNilNetworkConfig(networkName);

        validateAddress(l2NetworkConfig.l2CommonConfig.owner, "l2CommonConfig.owner");
        validateAddress(l2NetworkConfig.l2CommonConfig.admin, "l2CommonConfig.admin");

        const { tx: l2EthBridgeVaultImplementationDeploymentTx, address: l2EthBridgeVaultImplementationAddress } = await deployerAccount.deployContract({
            shardId: 1,
            bytecode: L2ETHBridgeVaultJson.default.bytecode as `0x${string}`,
            abi: L2ETHBridgeVaultJson.default.abi as Abi,
            args: [],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001)
        });

        await waitTillCompleted(deployerAccount.client, l2EthBridgeVaultImplementationDeploymentTx.hash, {
            waitTillMainShard: true
        });

        if (!l2EthBridgeVaultImplementationDeploymentTx || !l2EthBridgeVaultImplementationDeploymentTx.hash) {
            throw Error(`Invalid transaction output from deployContract call for L2ETHBridgeVault Contract`);
        }

        if (!l2EthBridgeVaultImplementationAddress) {
            throw Error(`Invalid address output from deployContract call for L2ETHBridgeVault Contract`);
        }

        l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultImplementation = getCheckSummedAddress(l2EthBridgeVaultImplementationAddress);

        const initData = encodeFunctionData({
            abi: L2ETHBridgeVaultJson.default.abi,
            functionName: "initialize",
            args: [l2NetworkConfig.l2CommonConfig.owner, l2NetworkConfig.l2CommonConfig.admin],
        });

        const { tx: proxyDeploymentTx, address: proxyAddress } = await deployerAccount.deployContract({
            shardId: 1,
            bytecode: TransparentUpgradeableProxy.default.bytecode as `0x${string}`,
            abi: TransparentUpgradeableProxy.default.abi as Abi,
            args: [l2EthBridgeVaultImplementationAddress, deployerAccount.address, initData],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await waitTillCompleted(deployerAccount.client, proxyDeploymentTx.hash, {
            waitTillMainShard: true
        });

        l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy = getCheckSummedAddress(proxyAddress);

        const proxyContractInstance = getContract({
            client: deployerAccount.client,
            abi: TransparentUpgradeableProxy.default.abi,
            address: l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy as `0x${string}`,
        });

        const proxyAdminAddress = await proxyContractInstance.read.fetchAdmin([]);
        l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.proxyAdmin = getCheckSummedAddress(proxyAdminAddress as `0x${string}`);

        const l2EthBridgeVaultProxyInstance = getContract({
            client: deployerAccount.client,
            abi: L2ETHBridgeVaultJson.default.abi as Abi,
            address: l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy as `0x${string}`
        });

        const l2ETHBridgeVaultOwner = await l2EthBridgeVaultProxyInstance.read.owner([]);
        if (l2ETHBridgeVaultOwner != l2NetworkConfig.l2CommonConfig.owner) {
            throw Error(`OwnerAddress in vaultContract: ${l2ETHBridgeVaultOwner} is incorrect, correct owner as per config: ${l2NetworkConfig.l2CommonConfig.owner}`);
        }

        const ethAmountTracker = await l2EthBridgeVaultProxyInstance.read.ethAmountTracker([]);
        if (ethAmountTracker != 0) {
            throw Error(`ethAmountTracker must be initializwd to 0 but it has value: ${ethAmountTracker} is incorrect`);
        }

        const l2ETHBridge = await l2EthBridgeVaultProxyInstance.read.l2ETHBridge([]);
        if (l2ETHBridge != ZeroAddress) {
            throw Error(`l2ETHBridge must be wired after deployment but it got initialised during proxyDeployment`);
        }

        const implementationAddressFromContract = await l2EthBridgeVaultProxyInstance.read.getImplementation([]);
        if (implementationAddressFromContract != l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultImplementation) {
            throw Error(`L2ETHBridgeVaultImplementation in Proxy is incorrect`);
        }

        const ownerFromContract = await l2EthBridgeVaultProxyInstance.read.owner([]);
        if (ownerFromContract != l2NetworkConfig.l2CommonConfig.owner) {
            throw Error(`Owner address in Proxy is incorrect`);
        }

        // Save the updated config
        saveNilNetworkConfig(networkName, l2NetworkConfig);
    });
