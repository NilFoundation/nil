import type { Abi } from "abitype";
import { task } from "hardhat/config";
import {
    convertEthToWei,
    waitTillCompleted,
    getContract,
} from "@nilfoundation/niljs";
import { loadNilSmartAccount } from "./nil-smart-account";
import { L2NetworkConfig, loadNilNetworkConfig, saveNilNetworkConfig } from "../deploy/config/config-helper";
import { encodeFunctionData } from "viem";
import { getCheckSummedAddress, validateAddress } from "../scripts/utils/validate-config";

// npx hardhat deploy-l2-eth-bridge --networkname local
task("deploy-l2-eth-bridge", "Deploys L2ETHBridge contract on Nil Chain")
    .addParam("networkname", "The network to use") // Mandatory parameter
    .setAction(async (taskArgs) => {

        // Dynamically load artifacts
        const L2ETHBridgeJson = await import("../artifacts/contracts/bridge/l2/L2ETHBridge.sol/L2ETHBridge.json");
        const TransparentUpgradeableProxy = await import("../artifacts/contracts/common/TransparentUpgradeableProxy.sol/MyTransparentUpgradeableProxy.json");
        const ProxyAdmin = await import("../artifacts/node_modules/@openzeppelin/contracts/proxy/transparent/ProxyAdmin.sol/ProxyAdmin.json");

        if (!L2ETHBridgeJson || !L2ETHBridgeJson.default || !L2ETHBridgeJson.default.abi || !L2ETHBridgeJson.default.bytecode) {
            throw Error(`Invalid L2ETHBridge ABI`);
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
        validateAddress(
            l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy,
            "l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy"
        );
        validateAddress(
            l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy,
            "l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy"
        );

        const { tx: l2EthBridgeImplementationDeploymentTx, address: l2EthBridgeImplementationAddress } = await deployerAccount.deployContract({
            shardId: 1,
            bytecode: L2ETHBridgeJson.default.bytecode as `0x${string}`,
            abi: L2ETHBridgeJson.default.abi as Abi,
            args: [],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });

        await waitTillCompleted(deployerAccount.client, l2EthBridgeImplementationDeploymentTx.hash, {
            waitTillMainShard: true
        });

        if (!l2EthBridgeImplementationDeploymentTx.hash) {
            throw Error(`Invalid transaction output from deployContract call for L2ETHBridge Contract`);
        }

        if (!l2EthBridgeImplementationAddress) {
            throw Error(`Invalid address output from deployContract call for L2ETHBridge Contract`);
        }

        l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeImplementation = getCheckSummedAddress(l2EthBridgeImplementationAddress);

        const initData = encodeFunctionData({
            abi: L2ETHBridgeJson.default.abi,
            functionName: "initialize",
            args: [l2NetworkConfig.l2CommonConfig.owner,
            l2NetworkConfig.l2CommonConfig.admin,
            l2NetworkConfig.l2BridgeMessengerConfig.l2BridgeMessengerContracts.l2BridgeMessengerProxy,
            l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy],
        });
        const { tx: l2EthBridgeProxyDeploymentTx, address: l2EthBridgeProxyAddress } = await deployerAccount.deployContract({
            shardId: 1,
            bytecode: TransparentUpgradeableProxy.default.bytecode as `0x${string}`,
            abi: TransparentUpgradeableProxy.default.abi as Abi,
            args: [l2EthBridgeImplementationAddress, deployerAccount.address, initData],
            salt: BigInt(Math.floor(Math.random() * 10000)),
            feeCredit: convertEthToWei(0.001),
        });
        await waitTillCompleted(deployerAccount.client, l2EthBridgeProxyDeploymentTx.hash, {
            waitTillMainShard: true
        });

        l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy = getCheckSummedAddress(l2EthBridgeProxyAddress);

        const proxyContractInsntace = getContract({
            client: deployerAccount.client,
            abi: TransparentUpgradeableProxy.default.abi,
            address: l2EthBridgeProxyAddress,
        });

        const proxyAdminAddress = await proxyContractInsntace.read.fetchAdmin([]);
        l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.proxyAdmin = getCheckSummedAddress(proxyAdminAddress as `0x${string}`);

        const l2EthBridgeProxyInstance = getContract({
            client: deployerAccount.client,
            abi: L2ETHBridgeJson.default.abi as Abi,
            address: l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeProxy as `0x${string}`
        });

        const l2ETHBridgeVault = await l2EthBridgeProxyInstance.read.l2ETHBridgeVault([]);
        if (l2ETHBridgeVault != l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy) {
            throw Error(`l2ETHBridgeVault was incorrectly initialised to address: ${l2ETHBridgeVault}. expectedValue: ${l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy}`);
        }

        const l2ETHBridgeOwner = await l2EthBridgeProxyInstance.read.owner([]);
        if (l2ETHBridgeOwner != l2NetworkConfig.l2CommonConfig.owner) {
            throw Error(`OwnerAddress in ETHBridgeContract: ${l2ETHBridgeOwner} is incorrect, correct owner as per config: ${l2NetworkConfig.l2CommonConfig.owner}`);
        }

        const implementationAddressFromContract = await l2EthBridgeProxyInstance.read.getImplementation([]);
        if (implementationAddressFromContract != l2NetworkConfig.l2ETHBridgeConfig.l2ETHBridgeContracts.l2ETHBridgeImplementation) {
            throw Error(`L2ETHBridgeImplementation in Proxy is incorrect`);
        }

        // Save the updated config
        saveNilNetworkConfig(networkName, l2NetworkConfig);
    });
