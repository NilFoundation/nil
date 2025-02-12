import { ethers, network } from "hardhat";
import { Contract } from "ethers";
import * as fs from "fs";
import * as path from "path";
import { isValidAddress, loadConfig } from "../../../deploy/config/config-helper";

// Load the ABI from the JSON file
const abiPath = path.join(__dirname, "../../artifacts/contracts/interfaces/INilAccessControl.sol/INilAccessControl.json");
const abi = JSON.parse(fs.readFileSync(abiPath, "utf8")).abi;

// Function to check if an address is a proposer
export async function isAProposer(proposerAddress: string) {
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
  const nilAccessControlInstance: Contract = new ethers.Contract(config.nilRollupProxy, abi, signer) as Contract;

  const isAProposerResponse = await nilAccessControlInstance.isAProposer(proposerAddress);

  console.log(`isAProposer Response is: ${JSON.stringify(isAProposerResponse)}`);

  // Convert the response to a boolean
  const isProposer = Boolean(isAProposerResponse);

  if (isProposer) {
    console.log(`account: ${proposerAddress} is a Proposer on network: ${networkName} for rollupContract: ${config.nilRollupProxy}`);
  } else {
    console.log(`account: ${proposerAddress} is not-a Proposer on network: ${networkName} for rollupContract: ${config.nilRollupProxy}`);
  }

  return isProposer;
}

// Main function to call the isAProposer function for an account
// async function main() {
//     const proposerAddress = '0x7A2f4530b5901AD1547AE892Bafe54c5201D1206';
//     await isAProposer(proposerAddress);
//   }
  
//   // npx hardhat run scripts/access-control/proposer/is-a-proposer.ts --network sepolia
//   main().catch((error) => {
//     console.error(error);
//     process.exit(1);
//   });