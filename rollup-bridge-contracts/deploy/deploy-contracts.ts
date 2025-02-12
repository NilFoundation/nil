import { DeployFunction } from "hardhat-deploy/types";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { ethers, upgrades, run } from "hardhat";
import { loadConfig, saveConfig, archiveConfig, isValidAddress, isValidBytes32, NetworkConfig, ZeroAddress } from './config/config-helper';

// npx hardhat deploy --network anvil --tags NilContracts
// npx hardhat deploy --network geth --tags NilContracts
// npx hardhat deploy --network sepolia --tags NilContracts
const deployNilContracts: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { deployments, getNamedAccounts, network } = hre;
  const { deploy } = deployments;
  const networkName = network.name;

  const { deployer } = await getNamedAccounts();

  console.log(`deployer address is: ${deployer}`);

  // dummy state root place holder, this will be replaced by a real value provided by team
  // all values can be set in via cli arguments or from a json file
  const genesisStateRootConst = ethers.encodeBytes32String("dummyStateRoot");
  console.log(`genesisStateRoot is: ${genesisStateRootConst}`);

  const config: NetworkConfig = loadConfig(networkName);

  //verify if the config object is not null and valid NetworkConfig
  if (!config) {
    throw new Error(`Invalid NetworkConfig for network: ${networkName}`);
  }

  // Validate configuration parameters
  if (!isValidAddress(config.nilRollupOwnerAddress)) {
    throw new Error("Invalid nilRollupOwnerAddress in config");
  }
  if (!isValidAddress(config.defaultAdminAddress)) {
    throw new Error("Invalid defaultAdminAddress in config");
  }
  if (!isValidAddress(config.proposerAddress)) {
    throw new Error("Invalid proposerAddress in config");
  }
  if (!isValidBytes32(config.genesisStateRoot)) {
    throw new Error("Invalid genesisStateRoot in config");
  }

  console.log(`Deploying NilVerifier to ${networkName}...`);

  // Check if NilVerifier is already deployed
  if (config.nilVerifier && isValidAddress(config.nilVerifier)) {
    console.log(`NilVerifier already deployed at: ${config.nilVerifier}`);
    archiveConfig(networkName, config);
  }

  const nilVerifier = await deploy("NilVerifier", {
    from: deployer,
    args: [],
    log: true,
  });

  console.log("NilVerifier deployed to:", nilVerifier.address);
  config.nilVerifier = nilVerifier.address;

  if (!isValidAddress(config.nilVerifier)) {
    throw new Error("Invalid nilVerifier address in config");
  }

  console.log("Deploying NilRollup with the account:", deployer);

  const nilVerifierAddress = config.nilVerifier;
  const l2ChainId = config.l2ChainId;
  const proposerAddress = config.proposerAddress;
  const ownerAddress = config.nilRollupOwnerAddress;
  const adminAddress = config.defaultAdminAddress;

  // config.proposerAddress = proposerAddress;
  // config.nilRollupOwnerAddress = ownerAddress;
  // config.defaultAdminAddress = adminAddress;

  console.log(`proposerAddress is: ${proposerAddress}`);
  console.log(`ownerAddress is: ${ownerAddress}`);
  console.log(`adminAddress is: ${adminAddress}`);

  // const genesisStateRoot = ethers.formatBytes32String(config.genesisStateRoot);
  // console.log(`genesisStateRoot is: ${genesisStateRoot}`);

  try {
    // Deploy NilRollup implementation
    const NilRollup = await ethers.getContractFactory("NilRollup");

    console.log(`Deploying NilRollupProxy contract with l2ChainId as: ${l2ChainId}`);

    const nilRollupProxy = await upgrades.deployProxy(NilRollup,
      [
        l2ChainId,
        ownerAddress, // _owner
        adminAddress, // _defaultAdmin
        nilVerifierAddress, // nilVerifier contract address
        proposerAddress, // proposer address
        config.genesisStateRoot
      ],
      { initializer: 'initialize' }
    );

    console.log(`NilRollup proxy deployed to: ${nilRollupProxy.target}`);

    const nilRollupProxyAddress = nilRollupProxy.target;
    config.nilRollupProxy = nilRollupProxyAddress;

    // query proxyAdmin address and implementation address
    const proxyAdminAddress = await getProxyAdminAddressWithRetry(nilRollupProxyAddress);
    console.log(`ProxyAdmin for proxy: ${nilRollupProxyAddress} is: ${proxyAdminAddress}`);
    config.proxyAdminAddress = proxyAdminAddress;

    if (proxyAdminAddress === ZeroAddress) {
      throw new Error("Invalid proxy admin address");
    }

    const implementationAddress = await upgrades.erc1967.getImplementationAddress(nilRollupProxyAddress);
    console.log(`Implementation address for proxy: ${nilRollupProxyAddress} is: ${implementationAddress}`);
    config.nilRollupImplementation = implementationAddress;

    if (implementationAddress === ZeroAddress) {
      throw new Error("Invalid implementation address");
    }

    // Query the proxy storage and assert if the input arguments are correctly set in the contract storage
    const nilRollup = NilRollup.attach(nilRollupProxyAddress);

    const storedL2ChainId = await nilRollup.l2ChainId();
    const storedOwnerAddress = await nilRollup.owner();
    const storedAdminAddress = await nilRollup.getRoleMember(await nilRollup.DEFAULT_ADMIN_ROLE(), 0);
    const storedNilVerifierAddress = await nilRollup.nilVerifierAddress();
    const storedProposerAddress = await nilRollup.getRoleMember(await nilRollup.PROPOSER_ROLE(), 0);
    const storedGenesisStateRoot = await nilRollup.batchInfoRecords("GENESIS_BATCH_INDEX").then((info: BatchInfo) => info.newStateRoot);

    console.log(`Stored l2ChainId: ${storedL2ChainId}`);
    console.log(`Stored ownerAddress: ${storedOwnerAddress}`);
    console.log(`Stored adminAddress: ${storedAdminAddress}`);
    console.log(`Stored nilVerifierAddress: ${storedNilVerifierAddress}`);
    console.log(`Stored proposerAddress: ${storedProposerAddress}`);
    console.log(`Stored genesisStateRoot: ${storedGenesisStateRoot}`);

    if (storedL2ChainId.toString() !== l2ChainId.toString()) {
      throw new Error("l2ChainId mismatch");
    }
    if (storedOwnerAddress.toLowerCase() !== ownerAddress.toLowerCase()) {
      throw new Error("ownerAddress mismatch");
    }
    if (storedAdminAddress.toLowerCase() !== adminAddress.toLowerCase()) {
      throw new Error("adminAddress mismatch");
    }
    if (storedNilVerifierAddress.toLowerCase() !== nilVerifierAddress.toLowerCase()) {
      throw new Error("nilVerifierAddress mismatch");
    }
    if (storedProposerAddress.toLowerCase() !== proposerAddress.toLowerCase()) {
      throw new Error("proposerAddress mismatch");
    }
    if (storedGenesisStateRoot.toLowerCase() !== config.genesisStateRoot.toLowerCase()) {
      throw new Error("genesisStateRoot mismatch");
    }

    // Save the updated config
    saveConfig(networkName, config);

    // check network and verify if its not geth or anvil
    // Skip verification if the network is local or anvil
    if (network.name !== "local" && network.name !== "anvil" && network.name !== "geth") {

      try {
        await verifyContractWithRetry(nilVerifier.address, []);
      } catch (error) {
        console.error("NilVerifier Verification failed after retries:", error);
      }

      try {
        await verifyContractWithRetry(nilRollupProxyAddress, []);
      } catch (error) {
        console.error("NilRollup Verification failed after retries:", error);
      }

    } else {
      console.log("Skipping verification on local or anvil network");
    }

  } catch (error) {
    console.error("Error during deployment:", error);
    process.exit(1);
  }
};

// Sleep for 5 seconds
function sleep(ms: number) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function getProxyAdminAddressWithRetry(nilRollupProxyAddress: string, retries: number = 10): Promise<string> {
  for (let i = 0; i < retries; i++) {
    const proxyAdminAddress = await upgrades.erc1967.getAdminAddress(nilRollupProxyAddress);

    console.log(`proxyAdminAddress for proxy: ${nilRollupProxyAddress} is extracted as: ${proxyAdminAddress}`);

    if (proxyAdminAddress !== ZeroAddress) {
      return proxyAdminAddress;
    }
    console.log(`ProxyAdmin address is zero. Retrying... (${i + 1}/${retries})`);
    await sleep(1000 * Math.pow(2, i)); // Exponential backoff delay
  }
  throw new Error('Failed to get ProxyAdmin address after multiple attempts');
}

async function verifyContractWithRetry(address: string, constructorArguments: any[], retries: number = 10): Promise<void> {
  for (let i = 0; i < retries; i++) {
    try {
      await run("verify:verify", {
        address,
        constructorArguments,
      });
      console.log(`Contract at ${address} verified successfully`);
      return;
    } catch (error) {
      console.error(`Verification failed for contract at ${address}:`, error);
      if (i < retries - 1) {
        console.log(`Retrying verification... (${i + 1}/${retries})`);
        await sleep(1000 * Math.pow(2, i)); // Exponential backoff delay
      } else {
        throw new Error(`Failed to verify contract at ${address} after ${retries} attempts`);
      }
    }
  }
}

export default deployNilContracts;
deployNilContracts.tags = ["NilContracts"];