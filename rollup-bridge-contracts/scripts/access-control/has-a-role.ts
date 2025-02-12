import { ethers, network } from "hardhat";
import { Contract } from "ethers";
import * as fs from "fs";
import * as path from "path";
import { loadConfig, isValidAddress } from "../../deploy/config/config-helper";
import { DEFAULT_ADMIN_ROLE, PROPOSER_ROLE_ADMIN } from "../utils/roles";

// Load the ABI from the JSON file
const abiPath = path.join(__dirname, "../../artifacts/contracts/NilAccessControl.sol/NilAccessControl.json");
const abi = JSON.parse(fs.readFileSync(abiPath, "utf8")).abi;

// npx hardhat run scripts/access-control/has-a-role.ts --network sepolia
export async function hasRole(roleHash: string, account: string) {

    const networkName = network.name;
    const config = loadConfig(networkName);

    // Validate configuration parameters
    if (!isValidAddress(config.nilRollupProxy)) {
        throw new Error("Invalid nilRollupProxy address in config");
    }

    // Get the signer (default account)
    const [signer] = await ethers.getSigners();

    //console.log(`nilRollupProxy on network: ${networkName} at address: ${config.nilRollupProxy}`);

    // Create a contract instance
    const nilRollupInstance = new ethers.Contract(config.nilRollupProxy, abi, signer) as Contract;

    const hasRoleResponse = await nilRollupInstance.hasRole(roleHash, account);

    // console.log(`hasRole output is: ${JSON.stringify(hasRoleResponse)}`);

    // if(hasRoleResponse) {
    //     console.log(`account: ${account} has Role: ${roleHash}`);
    // } else {
    //     console.log(`account: ${account} don't have role: ${roleHash}`);
    // }

    return hasRoleResponse;
}

// Main function to call the getRoleMembers function
// async function main() {
//     const account  = '0x658805a93Af995ccf5C2ab3B9B06302653289E68';
//     await hasRole(DEFAULT_ADMIN_ROLE, account);
// }

// main().catch((error) => {
//     console.error(error);
//     process.exit(1);
// });