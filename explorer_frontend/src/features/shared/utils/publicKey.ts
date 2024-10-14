import {
  type Hex,
  type RecoverPublicKeyParameters,
  type RecoverPublicKeyReturnType,
  hexToNumber,
  isHex,
  toHex,
} from "viem";

export async function recoverPublicKey({
  hash,
  signature,
}: RecoverPublicKeyParameters): Promise<RecoverPublicKeyReturnType> {
  const signatureHex = isHex(signature) ? signature : toHex(signature);
  const hashHex = isHex(hash) ? hash : toHex(hash);

  // Derive v = recoveryId + 27 from end of the signature (27 is added when signing the message)
  // The recoveryId represents the y-coordinate on the secp256k1 elliptic curve and can have a value [0, 1].
  let v = hexToNumber(`0x${signatureHex.slice(130)}`);
  if (v === 0 || v === 1) v += 27;

  const { secp256k1 } = await import("@noble/curves/secp256k1");
  const publicKey = secp256k1.Signature.fromCompact(signatureHex.substring(2, 130))
    .addRecoveryBit(v - 27)
    .recoverPublicKey(hashHex.substring(2))
    .toHex(true);
  return `0x${publicKey}`;
}

export const uncompressPublicKey = async (publicKey: Hex): Promise<Hex> => {
  const { secp256k1 } = await import("@noble/curves/secp256k1");
  const t = secp256k1.ProjectivePoint.fromHex(publicKey.substring(2)).toRawBytes(false);
  console.log(t);
  const publicKeyInstance = Buffer.from(t);
  return `0x${publicKeyInstance.toString("hex")}`;
};
