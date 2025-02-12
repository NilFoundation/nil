import { ethers, network } from "hardhat";
import { Contract } from "ethers";
import * as fs from "fs";
import * as path from "path";
import { loadConfig, isValidAddress } from "../../../deploy/config/config-helper";

// Load the ABI from the JSON file
const abiPath = path.join(__dirname, "../../../artifacts/contracts/NilRollup.sol/NilRollup.json");
const abi = JSON.parse(fs.readFileSync(abiPath, "utf8")).abi;

export async function getRollupPendingOwner() {
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
  
      const rollupProxyPendingOwner = await nilRollupInstance.pendingOwner();

      console.log(`rollupProxy has pending-owner: ${rollupProxyPendingOwner}`);
      
      return rollupProxyPendingOwner;
}

// Main function to call the getRollupPendingOwner function for an account
async function main() {
  await getRollupPendingOwner();
}

// npx hardhat run scripts/access-control/owner/get-pending-owner.ts --network sepolia
main().catch((error) => {
  console.error(error);
  process.exit(1);
});