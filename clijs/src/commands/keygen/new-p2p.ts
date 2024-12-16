import { BaseCommand } from "../../base.js";

import { generateKeyPair } from "@libp2p/crypto/keys";
import { peerIdFromPrivateKey } from "@libp2p/peer-id";
import { addHexPrefix, bytesToHex, type Hex } from "@nilfoundation/niljs";

async function generateSecp256k1Key(): Promise<{
  privateKey: Hex;
  publicKey: Hex;
  peerId: string;
}> {
  // Generate a Secp256k1 key pair
  const privateKey = await generateKeyPair("secp256k1");

  // Create a PeerId from the private key
  const peerId = peerIdFromPrivateKey(privateKey);

  // Convert the keys to Hex
  const privateKeyHex = addHexPrefix(bytesToHex(privateKey.raw));
  const publicKeyHex = addHexPrefix(bytesToHex(privateKey.publicKey.raw));

  // Return the key pair
  return {
    privateKey: privateKeyHex,
    publicKey: publicKeyHex,
    peerId: peerId.toString(),
  };
}

export default class KeygenNewP2p extends BaseCommand {
  static override description = "Generate a new p2p key";

  static override examples = ["<%= config.bin %> <%= command.id %>"];

  public async run(): Promise<Hex> {
    const { privateKey, publicKey, peerId } = await generateSecp256k1Key();
    if (this.quiet) {
      this.log(privateKey);
      this.log(publicKey);
      this.log(peerId);
    } else {
      this.log(`Private key: ${privateKey}`);
      this.log(`Public key: ${publicKey}`);
      this.log(`Identity: ${peerId}`);
    }
    return privateKey;
  }
}
