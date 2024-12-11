import { describe, expect } from "vitest";
import { CliTest } from "../../setup.js";

describe("keygen:new-p2p", () => {
  CliTest("runs keygen:new-p2p cmd", async ({ runCommand }) => {
    const { result } = await runCommand(["keygen", "new-p2p"]);
    expect(result).toBeTruthy();
  });
});
