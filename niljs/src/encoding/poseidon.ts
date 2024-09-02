import * as pkg from "@iden3/js-crypto";
let poseidon: typeof pkg.poseidon;
if (
  "default" in pkg &&
  typeof pkg.default === "object" &&
  pkg.default !== null &&
  "poseidon" in pkg.default &&
  typeof pkg.default.poseidon === "function"
) {
  poseidon = pkg.default.poseidon as typeof pkg.poseidon;
} else {
  poseidon = pkg.poseidon;
}

/**
 * Creates the Poseidon hash of the given bytes.
 * @param byes The bytes to hash.
 * @returns The Poseidon hash.
 */
const poseidonHash = (bytes: Uint8Array) => {
  return poseidon.hashBytesX(bytes, 16);
};

export { poseidonHash };
