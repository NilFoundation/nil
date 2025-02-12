import { ethers, network } from "hardhat";
import { Contract } from "ethers";
import * as fs from "fs";
import * as path from "path";
import { loadConfig, isValidAddress } from "../../../deploy/config/config-helper";

// Load the ABI from the JSON file
const abiPath = path.join(__dirname, "../../../artifacts/contracts/NilRollup.sol/NilRollup.json");
const abi = JSON.parse(fs.readFileSync(abiPath, "utf8")).abi;

export async function getRollupOwner() {
      const networkName = network.name;
      const config = loadConfig(networkName);
  
      // Validate configuration parameters
      if (!isValidAddress(config.nilRollupProxy)) {
          throw new Error("Invalid nilRollupProxy address in config");
      }
  
      // Get the signer (default account)
      const [signer] = await ethers.getSigners();
  
      console.log(`nilRollupProxy on network: ${networkName} at address: ${config.nilRollupProxy}`);
  
      // Create a contract instance
      const nilRollupInstance = new ethers.Contract(config.nilRollupProxy, abi, signer) as Contract;
  
      const rollupProxyOwner = await nilRollupInstance.owner();

      console.log(`rollupProxy has owner: ${rollupProxyOwner}`);
      
      return rollupProxyOwner;
}

// Main function to call the isAProposer function for an account
async function main() {
  await getRollupOwner();
}

// npx hardhat run scripts/access-control/owner/get-owner.ts --network sepolia
main().catch((error) => {
  console.error(error);
  process.exit(1);
});