import { ethers, network } from 'hardhat';
import { loadConfig } from '../../../deploy/config/config-helper';
import { hasRole } from '../has-a-role';
import { OWNER_ROLE } from '../../utils/roles';

// Function to check if an address is a proposer
export async function hasOwnershipRole(account: string) {
    const hasOwnershipRole = await hasRole(OWNER_ROLE, account);

    // Convert the response to a boolean
    const hasOwnershipRoleIndicator = Boolean(hasOwnershipRole);

    const networkName = network.name;
    const config = loadConfig(networkName);

    if (hasOwnershipRoleIndicator) {
        console.log(
            `account: ${account} is an owner for rollupContract: ${config.nilRollupProxy} on network: ${networkName}`,
        );
    } else {
        console.log(
            `account: ${account} doesnot have owner-role for rollupContract: ${config.nilRollupProxy} on network: ${networkName}`,
        );
    }
}

// Main function to call the isAProposer function for an account
async function main() {
    //const account = '0x66bFaD51E02513C5B6bEfe1Acc9a31Cb6eE152F1';
    const account = '0x658805a93Af995ccf5C2ab3B9B06302653289E68';
    await hasOwnershipRole(account);
}

// npx hardhat run scripts/access-control/owner/has-ownership-role.ts --network sepolia
main().catch((error) => {
    console.error(error);
    process.exit(1);
});
