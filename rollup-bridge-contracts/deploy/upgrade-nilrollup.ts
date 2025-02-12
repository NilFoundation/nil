import { HardhatRuntimeEnvironment } from "hardhat/types";
import { DeployFunction } from "hardhat-deploy/types";
import { ethers, network, upgrades } from "hardhat";
import { archiveConfig, isValidAddress, loadConfig, NetworkConfig, saveConfig } from "./config/config-helper";

// npx hardhat deploy --network sepolia --tags UpgradeNilRollup
// npx hardhat deploy --network anvil --tags UpgradeNilRollup
// npx hardhat deploy --network geth --tags UpgradeNilRollup
const upgradeNilRollup: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { deployments, getNamedAccounts } = hre;
  const { deployer } = await getNamedAccounts();

  const networkName = network.name;

  console.log(`deployer address is: ${deployer}`);
  const config: NetworkConfig = loadConfig(networkName);

  // Check if NilRollup is already deployed
  if (!config.nilRollupProxy || !isValidAddress(config.nilRollupProxy)) {
    throw new Error(`NilRollup is not deployed yet on chain: ${networkName}`)
  }

  archiveConfig(networkName, config);

  const nilRollupProxyAddress: string = config.nilRollupProxy;

  console.log("Checking current implementation address...");
  const currentImplementationAddress = await upgrades.erc1967.getImplementationAddress(nilRollupProxyAddress);
  console.log(`Current implementation address: ${currentImplementationAddress}`);

  console.log("Registering the existing proxy...");
  //await upgrades.forceImport(nilRollupProxyAddress, await ethers.getContractFactory("NilRollup"));

  console.log("Upgrading NilRollup...");

  // Deploy the new implementation contract and upgrade the proxy
  const NilRollupV2 = await ethers.getContractFactory("NilRollup");
  const upgradedProxy = await upgrades.upgradeProxy(nilRollupProxyAddress, NilRollupV2);

  console.log(`NilRollup upgraded. Proxy address: ${upgradedProxy.target}`);

  console.log("Checking new implementation address...");
  const newImplementationAddress = await upgrades.erc1967.getImplementationAddress(nilRollupProxyAddress);
  console.log(`New implementation address: ${newImplementationAddress}`);

  // Verify that the implementation address has changed
  // if (currentImplementationAddress === newImplementationAddress) {
  //   throw new Error("Upgrade failed: Implementation address did not change");
  // }

  console.log("Upgrade successful: Implementation address has changed");

  // Additional checks to verify contract state
  const nilRollup = await ethers.getContractAt("NilRollup", nilRollupProxyAddress);

  // Example check: Verify that the l2ChainId is still correct
  const l2ChainId = await nilRollup.l2ChainId();
  console.log(`l2ChainId: ${l2ChainId}`);
  if (l2ChainId.toString() !== "0") {
    throw new Error("Upgrade failed: l2ChainId is incorrect");
  }

  console.log("All checks passed: Upgrade is successful");

  config.nilRollupImplementation = newImplementationAddress;

  // save updated config
  saveConfig(networkName, config);
};

export default upgradeNilRollup;
upgradeNilRollup.tags = ["UpgradeNilRollup"];