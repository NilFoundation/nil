import type { Abi } from "abitype";
import invariant from "tiny-invariant";
import { encodeFunctionData } from "viem";
import { bytesToHex, hexToBytes } from "../encoding/index.js";
import type { Hex } from "../types/index.js";
import { addHexPrefix } from "./hex.js";

const refineSalt = (salt: Uint8Array | bigint): Uint8Array => {
  if (typeof salt === "bigint") {
    return hexToBytes(addHexPrefix(salt.toString(16).padStart(64, "0"))).slice(0, 32);
  }

  invariant(salt.length === 32, "Salt must be 32 bytes");

  return salt;
};

const refineBigintSalt = (salt: Uint8Array | bigint): bigint => {
  if (typeof salt === "bigint") {
    return salt;
  }

  return BigInt(addHexPrefix(bytesToHex(salt)));
};

const refineCompressedPublicKey = (pubkey: Uint8Array | `0x${string}`): Uint8Array => {
  const res = typeof pubkey === "string" ? hexToBytes(pubkey) : pubkey;
  invariant(res.length === 33, "Invalid pubkey length");

  return res;
};

const refineFunctionHexData = ({
  data,
  abi,
  functionName,
  args,
}: {
  data?: Uint8Array | Hex;
  abi?: Abi;
  functionName?: string;
  args?: unknown[];
}): Hex => {
  if (!data && !abi) {
    return "0x";
  }
  invariant(!(data && abi), "ABI and data cannot be provided together");
  if (data) {
    return typeof data === "string" ? data : bytesToHex(data);
  }
  invariant(abi && functionName, "ABI and functionName is required");
  return encodeFunctionData({
    abi,
    functionName,
    args: args || [],
  });
};

export { refineSalt, refineCompressedPublicKey, refineFunctionHexData, refineBigintSalt };
