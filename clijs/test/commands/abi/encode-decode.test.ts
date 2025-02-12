import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";

// To run this test you need to run the nild:
// nild run --http-port 8529
// TODO: Setup nild automatically before running the tests
describe("abi:encode-decode", () => {
  CliTest("tests abi encoding and decoding", async ({ runCommand }) => {
    const encoded = (
      await runCommand([
        "abi",
        "encode",
        "-p",
        "../nil/contracts/compiled/tests/Counter.abi",
        "add",
        "1000",
      ])
    ).result as string;
    expect(encoded).toHaveLength(74);

    const decoded = (
      await runCommand([
        "abi",
        "decode",
        "-p",
        "../nil/contracts/compiled/tests/Counter.abi",
        encoded,
      ])
    ).result;
    // @ts-ignore
    expect(decoded.functionName).eq("add");
    // @ts-ignore
    expect(decoded.args as number[]).contains(1000);
  });
});
