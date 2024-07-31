import {
  hashObjectToUint8Array,
  setHasher,
  uint8ArrayToHashObject,
} from "@chainsafe/persistent-merkle-tree";
import {
  BooleanType,
  ByteListType,
  ByteVectorType,
  ContainerType,
  UintBigintType,
  UintNumberType,
} from "@chainsafe/ssz";
import { concatBytes } from "@noble/curves/abstract/utils";
import { poseidonHash } from "./poseidon.js";

setHasher({
  digest64(a, b) {
    const hash = poseidonHash(concatBytes(a, b));
    const arr = new Uint8Array(32);
    for (let i = 0; i < 32; i++) {
      // Shift the BigInt to the right by 8 * i bits, then take the 8 least significant bits
      arr[31 - i] = Number((hash >> BigInt(i * 8)) & BigInt(0xff));
    }
    return arr;
  },
  digest64HashObjects(a, b) {
    const hash = poseidonHash(
      concatBytes(hashObjectToUint8Array(a), hashObjectToUint8Array(b)),
    );
    const arr = new Uint8Array(32);
    for (let i = 0; i < 32; i++) {
      // Shift the BigInt to the right by 8 * i bits, then take the 8 least significant bits
      arr[31 - i] = Number((hash >> BigInt(i * 8)) & BigInt(0xff));
    }
    return uint8ArrayToHashObject(arr);
  },
});

/**
 * The basic types used in the library.
 *
 */
const basicTypes = {
  Uint8: new UintNumberType(1),
  Uint32: new UintNumberType(4),
  Uint64: new UintNumberType(8),
  UintBn256: new UintBigintType(32),
  Bool: new BooleanType(),
};

/**
 * The const representing a byte vector with 20 elements.
 *
 */
const Bytes20 = new ByteVectorType(20);

/**
 * The SSZ schema for a message object.
 */
const SszMessageSchema = new ContainerType({
  deploy: basicTypes.Bool,
  to: Bytes20,
  chainId: basicTypes.Uint64,
  seqno: basicTypes.Uint64,
  data: new ByteListType(24576),
});

/**
 * SSZ schema for a signed message object. Includes auth data in addition to all other message fields.
 */
const SszSignedMessageSchema = new ContainerType({
  ...SszMessageSchema.fields,
  authData: new ByteListType(256),
});

export { SszMessageSchema, SszSignedMessageSchema };
