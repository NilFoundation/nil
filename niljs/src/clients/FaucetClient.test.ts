import { defaultAddress } from "../../test/mocks/address.js";
import { FaucetClient, addHexPrefix } from "../index.js";
import { MockTransport } from "../transport/MockTransport.js";

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
    walletAddress: addHexPrefix(defaultAddress),
    faucetAddress: addHexPrefix(defaultAddress),
    amount: 100,
  });

  expect(fn).toHaveBeenCalledOnce();
  expect(fn).toHaveBeenLastCalledWith({
    method: "faucet_topUpViaFaucet",
    params: [addHexPrefix(defaultAddress), addHexPrefix(defaultAddress), 100],
  });
});
