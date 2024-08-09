import { accounts } from "../../test/mocks/accounts.js";
import { bytesToHex } from "../index.js";
import { LocalECDSAKeySigner } from "./LocalECDSAKeySigner.js";

test("getPublicKey", async () => {
  const privateKey = accounts[0].privateKey;
  const signer = new LocalECDSAKeySigner({ privateKey });

  const publicKey = await signer.getPublicKey();
  expect(bytesToHex(publicKey)).toBe(accounts[0].publicKey);
});

test("getAddress", async () => {
  const privateKey = accounts[0].privateKey;
  const signer = new LocalECDSAKeySigner({ privateKey });

  const address = await signer.getAddress(accounts[0].shardId);

  expect(address).toBeDefined();
});

test("LocalKeySigner should throw error if invalid private key is provided", async () => {
  /**
   * The private key is invalid.
   */
  const privateKey = "0x0";
  expect(() => new LocalECDSAKeySigner({ privateKey })).toThrowError();
});

test("sign", async () => {
  const privateKey = accounts[0].privateKey;
  const signer = new LocalECDSAKeySigner({ privateKey });
  const message = Uint8Array.from([0, 1, 2, 3, 4, 5, 6, 7, 8, 9]);

  const signature = await signer.sign(message);

  expect(signature).toBeDefined();
});
