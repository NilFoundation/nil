import { defaultAddress } from "../../test/mocks/address.js";
import { addHexPrefix } from "../index.js";
import { MockTransport } from "../transport/MockTransport.js";
import { PublicClient } from "./PublicClient.js";

test("getBlockByHash", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue({});
  const client = new PublicClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });
  await client.getBlockByHash("0x158c4be17b52b92dc03cef7e8cd9cec64c6413175df3cce9f6ae1fb0d12106fa");
  expect(fn).toHaveBeenCalledOnce();

  expect(fn).toHaveBeenLastCalledWith({
    method: "eth_getBlockByHash",
    params: [1, "0x158c4be17b52b92dc03cef7e8cd9cec64c6413175df3cce9f6ae1fb0d12106fa", false],
  });
});

test("getBlockByNumber", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue({});
  const client = new PublicClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });
  await client.getBlockByNumber("0x1b4");
  expect(fn).toHaveBeenCalledOnce();
  expect(fn).toHaveBeenLastCalledWith({
    method: "eth_getBlockByNumber",
    params: [1, "0x1b4", false],
  });
});

test("getBlockMessageCountByNumber", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue(1);
  const client = new PublicClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });
  const count = await client.getBlockMessageCountByNumber("0x1b4", 1);

  expect(count).toBeDefined();
  expect(fn).toHaveBeenCalledOnce();
  expect(fn).toHaveBeenLastCalledWith({
    method: "eth_getBlockTransactionCountByNumber",
    params: [1, "0x1b4"],
  });
});

test("getBlockMessageCountByHash", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue(1);
  const client = new PublicClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });
  const count = await client.getBlockMessageCountByHash(
    "0x158c4be17b52b92dc03cef7e8cd9cec64c6413175df3cce9f6ae1fb0d12106fa",
  );

  expect(count).toBeDefined();
  expect(fn).toHaveBeenCalledOnce();
  expect(fn).toHaveBeenLastCalledWith({
    method: "eth_getBlockTransactionCountByHash",
    params: [1, "0x158c4be17b52b92dc03cef7e8cd9cec64c6413175df3cce9f6ae1fb0d12106fa"],
  });
});

test("getCode", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue("0x");
  const client = new PublicClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });
  const code = await client.getCode(addHexPrefix(defaultAddress), "latest");

  expect(code).toHaveLength(0);
  expect(fn).toHaveBeenCalledOnce();
  expect(fn).toHaveBeenLastCalledWith({
    method: "eth_getCode",
    params: [addHexPrefix(defaultAddress), "latest"],
  });
});

test("getMessageCount", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue("0x1");
  const client = new PublicClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });
  const count = await client.getMessageCount(addHexPrefix(defaultAddress), "latest");
  expect(count).toBe(1);
  expect(fn).toHaveBeenCalledOnce();
  expect(fn).toHaveBeenLastCalledWith({
    method: "eth_getTransactionCount",
    params: [addHexPrefix(defaultAddress), "latest"],
  });
});

test("getBalance", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue("0x100");
  const client = new PublicClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });
  const balance = await client.getBalance(addHexPrefix(defaultAddress));

  expect(balance).toBeDefined();
  expect(balance).toBe(256n);
  expect(fn).toHaveBeenCalledOnce();
  expect(fn).toHaveBeenLastCalledWith({
    method: "eth_getBalance",
    params: [addHexPrefix(defaultAddress), "latest"],
  });

  await client.getBalance(addHexPrefix(defaultAddress), "pending");

  expect(fn).toHaveBeenLastCalledWith({
    method: "eth_getBalance",
    params: [addHexPrefix(defaultAddress), "pending"],
  });
});

test("getMessageByHash", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue({
    value: "0x100",
    gasLimit: 100,
    gasUsed: "0x100",
    seqno: "0x100",
    index: "0x0",
  });
  const client = new PublicClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });
  const message = await client.getMessageByHash(
    "0x158c4be17b52b92dc03cef7e8cd9cec64c6413175df3cce9f6ae1fb0d12106fa",
  );

  expect(message).toBeDefined();
  expect(fn).toHaveBeenCalledOnce();
  expect(fn).toHaveBeenLastCalledWith({
    method: "eth_getInMessageByHash",
    params: [1, "0x158c4be17b52b92dc03cef7e8cd9cec64c6413175df3cce9f6ae1fb0d12106fa"],
  });
});

test("getMessageReceiptByHash", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue({
    gasPrice: "0x100",
    gasUsed: "0x100",
  });
  const client = new PublicClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });
  const receipt = await client.getMessageReceiptByHash(
    "0x158c4be17b52b92dc03cef7e8cd9cec64c6413175df3cce9f6ae1fb0d12106fa",
  );

  expect(receipt).toBeDefined();
  expect(fn).toHaveBeenCalledOnce();
  expect(fn).toHaveBeenLastCalledWith({
    method: "eth_getInMessageReceipt",
    params: [1, "0x158c4be17b52b92dc03cef7e8cd9cec64c6413175df3cce9f6ae1fb0d12106fa"],
  });
});

test("getGasPrice", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue("0x100");
  const client = new PublicClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });
  const gasPrice = await client.getGasPrice(1);
  expect(gasPrice).toBeDefined();
  expect(gasPrice).toBe(256n);
  expect(fn).toHaveBeenCalledOnce();
  expect(fn).toHaveBeenLastCalledWith({
    method: "eth_gasPrice",
    params: [1],
  });
});

test("estimateGasLimit", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue("0x100");
  const client = new PublicClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });
  const gasLimit = await client.estimateGasLimit();

  expect(gasLimit).toBeDefined();
});

test("chainId", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue("0x1");
  const client = new PublicClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });
  const chainId = await client.chainId();

  expect(chainId).toBeDefined();
  expect(chainId).toBe(1);
  expect(fn).toHaveBeenCalledOnce();
  expect(fn).toHaveBeenLastCalledWith({
    method: "eth_chainId",
    params: [],
  });
});

test("getCurrencies", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue({});
  const client = new PublicClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });
  const currencies = await client.getCurrencies(addHexPrefix(defaultAddress), "latest");

  expect(currencies).toBeDefined();
  expect(currencies).toBeInstanceOf(Object);
  expect(fn).toHaveBeenCalledOnce();
  expect(fn).toHaveBeenLastCalledWith({
    method: "eth_getCurrencies",
    params: [addHexPrefix(defaultAddress), "latest"],
  });
});
