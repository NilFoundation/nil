import { keccak256, toUtf8Bytes } from "ethers";

/**
 * Enum for Role Identifiers
 */
export enum Roles {
  OWNER_ROLE = "OWNER_ROLE",
  PROPOSER_ROLE = "PROPOSER_ROLE",
  PROPOSER_ROLE_ADMIN = "PROPOSER_ROLE_ADMIN",
  DEFAULT_ADMIN_ROLE = "DEFAULT_ADMIN_ROLE",
}

/**
 * Helper function to get the keccak256 hash of a role
 * @param role - The role as a string
 * @returns keccak256 hash of the role
 */
export function getRoleHash(role: Roles): string {
  if (role === Roles.DEFAULT_ADMIN_ROLE) {
    return "0x0000000000000000000000000000000000000000000000000000000000000000";
  }
  return keccak256(toUtf8Bytes(role));
}

// Exporting role hashes
export const OWNER_ROLE = getRoleHash(Roles.OWNER_ROLE);
export const PROPOSER_ROLE = getRoleHash(Roles.PROPOSER_ROLE);
export const PROPOSER_ROLE_ADMIN = getRoleHash(Roles.PROPOSER_ROLE_ADMIN);
export const DEFAULT_ADMIN_ROLE = getRoleHash(Roles.DEFAULT_ADMIN_ROLE);