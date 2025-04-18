import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";

describe("config:init", () => {
  CliTest("initializes a new config file", async ({ runCommand }) => {
    const { result } = await runCommand(["config", "init"]);

    expect(result).toBeTypeOf("string");
    expect(result).toBe("config initialized");
  });
});
