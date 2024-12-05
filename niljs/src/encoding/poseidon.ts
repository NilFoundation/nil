import { poseidon16 } from "poseidon-lite";

const SPONGE_CHUNK_SIZE = 32;

function beBuff2int(buff: Uint8Array) {
  let res = BigInt(0);
  let i = buff.length;
  let offset = 0;
  const buffV = new DataView(buff.buffer, buff.byteOffset, buff.byteLength);
  while (i > 0) {
    if (i >= 4) {
      i -= 4;
      res += BigInt(buffV.getUint32(i)) << BigInt(offset * 8);
      offset += 4;
    } else if (i >= 2) {
      i -= 2;
      res += BigInt(buffV.getUint16(i)) << BigInt(offset * 8);
      offset += 2;
    } else {
      i -= 1;
      res += BigInt(buffV.getUint8(i)) << BigInt(offset * 8);
      offset += 1;
    }
  }
  return res;
}

const hashBytes = (msg: Uint8Array): bigint => {
  const frameSize = 16;
  const inputs = new Array(frameSize).fill(BigInt(0));
  let dirty = false;
  let hash;
  let k = 0;
  for (let i = 0; i < parseInt(`${msg.length / SPONGE_CHUNK_SIZE}`); i += 1) {
      dirty = true;
      
      inputs[k] = beBuff2int(msg.slice(SPONGE_CHUNK_SIZE * i, SPONGE_CHUNK_SIZE * (i + 1)));
      if (k === frameSize - 1) {
          hash = poseidon16(inputs);
          dirty = false;
          inputs[0] = hash;
          inputs.fill(BigInt(0), 1, SPONGE_CHUNK_SIZE);
          for (let j = 1; j < frameSize; j += 1) {
              inputs[j] = BigInt(0);
          }
          k = 1;
      }
      else {
          k += 1;
      }
  }
  if (msg.length % SPONGE_CHUNK_SIZE != 0) {
      const buff = new Uint8Array(SPONGE_CHUNK_SIZE);
      const slice = msg.slice(parseInt(`${msg.length / SPONGE_CHUNK_SIZE}`) * SPONGE_CHUNK_SIZE);
      slice.forEach((v, idx) => {
          buff[idx] = v;
      });
      inputs[k] = beBuff2int(buff);
      dirty = true;
  }
  if (dirty) {
      // we haven't hashed something in the main sponge loop and need to do hash here
      hash = poseidon16(inputs);
  }
  return hash || 0n;
}

export const poseidonHash = hashBytes;