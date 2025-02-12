import { ethers, network } from "hardhat";
import { Contract } from "ethers";
import * as fs from "fs";
import * as path from "path";
import { loadConfig, isValidAddress } from "../../deploy/config/config-helper";
import { PROPOSER_ROLE_ADMIN } from "../utils/roles";

// Load the ABI from the JSON file
const abiPath = path.join(__dirname, "../../artifacts/contracts/NilAccessControl.sol/NilAccessControl.json");
const abi = JSON.parse(fs.readFileSync(abiPath, "utf8")).abi;

// npx hardhat run scripts/access-control/get-role-members.ts --network sepolia
export async function getRoleMembers(roleHash: string) {

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

    const roleMembers = await nilRollupInstance.getRoleMembers(roleHash);
    return roleMembers;
}

// Main function to call the getRoleMembers function
// async function main() {
//     await getRoleMembers();
// }

// main().catch((error) => {
//     console.error(error);
//     process.exit(1);
// });