import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";

describe("abi:encode-decode", () => {
  CliTest("tests abi encoding and decoding", async ({ runCommand }) => {
    const encoded = await runCommand([
      "abi",
      "encode",
      "-p",
      "./test/contracts/Counter/Counter.abi",
      "increment",
    ]);
    expect(encoded.result).toHaveLength(10);
    expect(encoded.stdout).toHaveLength(11);

    const decoded = await runCommand([
      "abi",
      "decode",
      "-p",
      "./test/contracts/Counter/Counter.abi",
      encoded.result as string,
    ]);
    // @ts-ignore
    expect(decoded.result.functionName).eq("increment");
    // @ts-ignore
    expect(decoded.result.args as number[]).toBeUndefined();
    expect(decoded.stdout).contains("increment");
  });
});
