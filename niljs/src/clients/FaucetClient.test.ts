import { defaultAddress } from "../../test/mocks/address.js";
import { toHex } from "../encoding/toHex.js";
import { MockTransport } from "../transport/MockTransport.js";
import { addHexPrefix } from "../utils/hex.js";
import { FaucetClient } from "./FaucetClient.js";

test("getAllFaucets", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue({});
  const client = new FaucetClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });

  await client.getAllFaucets();

  expect(fn).toHaveBeenCalledOnce();
  expect(fn).toHaveBeenLastCalledWith({
    method: "faucet_getFaucets",
    params: [],
  });
});

test("topUp", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue({});
  const client = new FaucetClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });

  await client.topUp({
    smartAccountAddress: addHexPrefix(defaultAddress),
    faucetAddress: addHexPrefix(defaultAddress),
    amount: 100n,
  });

  expect(fn).toHaveBeenCalledOnce();
  expect(fn).toHaveBeenLastCalledWith({
    method: "faucet_topUpViaFaucet",
    params: [addHexPrefix(defaultAddress), addHexPrefix(defaultAddress), toHex(100n)],
  });
});
