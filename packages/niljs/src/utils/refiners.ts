import invariant from "tiny-invariant";
import { hexToBytes } from "../index.js";
import { addHexPrefix } from "./hex.js";

const refineSalt = (salt: Uint8Array | bigint): Uint8Array => {
  if (typeof salt === "bigint") {
    return hexToBytes(addHexPrefix(salt.toString(16).padStart(64, "0"))).slice(
      0,
      32,
    );
  }

  invariant(salt.length === 32, "Salt must be 32 bytes");

  return salt;
};

const refineCompressedPublicKey = (
  pubkey: Uint8Array | `0x${string}`,
): Uint8Array => {
  const res = typeof pubkey === "string" ? hexToBytes(pubkey) : pubkey;
  invariant(pubkey.length === 33, "Invalid pubkey length");

  return res;
};

export { refineSalt, refineCompressedPublicKey };
