import { ethers, network } from "hardhat";
import { Contract } from "ethers";
import * as fs from "fs";
import * as path from "path";
import { isValidAddress, loadConfig } from "../../../deploy/config/config-helper";
import { hasRole } from "../has-a-role";
import { DEFAULT_ADMIN_ROLE } from "../../utils/roles";

// Function to check if an address is a Admin
export async function isAnAdmin(account: string) : Promise<Boolean> {
  const isAnAdminResponse = await hasRole(DEFAULT_ADMIN_ROLE, account)

  //console.log(`isAnAdminResponse Response is: ${JSON.stringify(isAnAdminResponse)}`);

  // Convert the response to a boolean
  const isAnAdminIndicator = Boolean(isAnAdminResponse);

  //console.log(`isAnAdmin is: ${JSON.stringify(isAnAdmin)}`);
  // const networkName = network.name;
  // const config = loadConfig(networkName);
  // if (isAnAdminIndicator) {
  //   console.log(`account: ${account} is an admin on network: ${networkName} for rollupContract: ${config.nilRollupProxy}`);
  // } else {
  //   console.log(`account: ${account} is not-an admin on network: ${networkName} for rollupContract: ${config.nilRollupProxy}`);
  // }

  return isAnAdminIndicator;
}

// Main function to call the isAnAdmin function for an account
// async function main() {
//     const account = '0x7A2f4530b5901AD1547AE892Bafe54c5201D1206';
//     await isAnAdmin(account);
//   }
  
//   // npx hardhat run scripts/access-control/admin/is-an-admin.ts --network sepolia
//   main().catch((error) => {
//     console.error(error);
//     process.exit(1);
//   });