import { hexToBigInt, hexToBytes, hexToNumber } from "./fromHex.js";

test("hexToBigInt", () => {
  expect(hexToBigInt("0x7b")).toBe(123n);
});

test("hexToNumber", () => {
  expect(hexToNumber("0x7b")).toBe(123);
});

test("hexToBytes", () => {
  expect(hexToBytes("0x7b")).toEqual(Uint8Array.from([123]));
});
