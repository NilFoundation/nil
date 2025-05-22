import { loadL1NetworkConfig } from '../../../deploy/config/config-helper';
import { hasRole } from '../has-a-role';
import { OWNER_ROLE } from '../../utils/roles';

export async function hasOwnershipRole(account: string) {
    // Lazy import inside the function
    // @ts-ignore
    const { ethers, network } = await import('hardhat');
    const hasOwnershipRole = await hasRole(OWNER_ROLE, account);

    const hasOwnershipRoleIndicator = Boolean(hasOwnershipRole);

    const networkName = network.name;
    const config = loadL1NetworkConfig(networkName);

    if (hasOwnershipRoleIndicator) {
        console.log(
            `account: ${account} is an owner for rollupContract: ${config.nilRollup.nilRollupContracts.nilRollupProxy} on network: ${networkName}`,
        );
    } else {
        console.log(
            `account: ${account} doesnot have owner-role for rollupContract: ${config.nilRollup.nilRollupContracts.nilRollupProxy} on network: ${networkName}`,
        );
    }
}

// Main function to call the isAProposer function for an account
async function main() {
    const account = '0x658805a93Af995ccf5C2ab3B9B06302653289E68';
    await hasOwnershipRole(account);
}

// npx hardhat run scripts/access-control/owner/has-ownership-role.ts --network sepolia
main().catch((error) => {
    console.error(error);
    process.exit(1);
});
