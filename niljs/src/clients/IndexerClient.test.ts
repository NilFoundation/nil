import { defaultAddress } from "../../test/mocks/address.js";
import { MockTransport } from "../transport/MockTransport.js";
import { addHexPrefix } from "../utils/hex.js";
import { AddressActionKind, AddressActionStatus, IndexerClient } from "./IndexerClient.js";

test("getAddressActions", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue([
    {
      hash: "0x158c4be17b52b92dc03cef7e8cd9cec64c6413175df3cce9f6ae1fb0d12106fa",
      from: addHexPrefix(defaultAddress),
      to: addHexPrefix(defaultAddress),
      amount: BigInt(1),
      timestamp: 1000,
      blockId: 200,
      type: AddressActionKind.ReceiveEth,
      status: AddressActionStatus.Success,
    },
  ]);
  const client = new IndexerClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });
  const actions = await client.getAddressActions(addHexPrefix(defaultAddress));

  expect(actions).toBeDefined();
  expect(fn).toHaveBeenCalledOnce();
  expect(fn).toHaveBeenLastCalledWith({
    method: "indexer_getAddressActions",
    params: [addHexPrefix(defaultAddress), 0],
  });
});
