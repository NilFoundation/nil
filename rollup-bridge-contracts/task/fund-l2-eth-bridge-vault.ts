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
import { decodeFunctionResult, encodeFunctionData } from "viem";
import { getCheckSummedAddress, validateAddress } from "../scripts/utils/validate-config";
import { ZeroAddress } from "ethers";

// npx hardhat fund-l2-eth-bridge-vault --networkname local
task("fund-l2-eth-bridge-vault", "funds L2ETHBridgeVault contract on Nil Chain")
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

        const rpcEndpoint = process.env.NIL_RPC_ENDPOINT as string;
        const faucetClient = new FaucetClient({
            transport: new HttpTransport({ endpoint: rpcEndpoint }),
        });

        const balanceBefFunding = await deployerAccount.client.getBalance(l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy as `0x${string}`);

        const topUpFaucet = await faucetClient.topUp({
            smartAccountAddress: l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy as `0x${string}`,
            amount: convertEthToWei(200),
            faucetAddress: process.env.NIL as `0x${string}`,
        });

        const fundL2ETHBrodgeVaultTxnReceipts: ProcessedReceipt[] = await waitTillCompleted(deployerAccount.client, topUpFaucet);

        // check the first element in the ProcessedReceipt and verify if it is successful
        if (!fundL2ETHBrodgeVaultTxnReceipts[0].success) {
            throw Error(`Failed to fund L2ETHBridgeVault: ${l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy}`);
        }

        const balanceAfterFunding = await deployerAccount.client.getBalance(l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy as `0x${string}`);

        console.log(`Successfully funded L2ETHBridgeVault: ${l2NetworkConfig.l2ETHBridgeVaultConfig.l2ETHBridgeVaultContracts.l2ETHBridgeVaultProxy} and it has balance: ${balanceAfterFunding}`)
    });
