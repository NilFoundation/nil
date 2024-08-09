import { bytesToString } from "./fromBytes.js";

test("bytesToString", () => {
  const bytes = new Uint8Array([104, 101, 108, 108, 111]);

  const str = bytesToString(bytes);

  expect(str).toBe("hello");
});
