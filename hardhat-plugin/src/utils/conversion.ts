export function hexStringToUint8Array(hexString: string): Uint8Array {
  const cleanHexString = hexString.startsWith("0x") ? hexString.slice(2) : hexString;
  const numBytes = cleanHexString.length / 2;
  const byteArray = new Uint8Array(numBytes);

  for (let i = 0; i < numBytes; i++) {
    byteArray[i] = Number.parseInt(cleanHexString.substring(i * 2, i * 2 + 2), 16);
  }

  return byteArray;
}

export function shardNumber(address: string): number {
  let formattedAddress = address;
  if (address.startsWith("0x")) {
    formattedAddress = address.slice(2);
  }
  const shardHex = formattedAddress.slice(0, 4);
  return Number.parseInt(shardHex, 16);
}
