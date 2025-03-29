import { MockTransport } from "../transport/MockTransport.js";
import { CometaClient } from "./CometaClient.js";
import type { ContractData } from "./types/CometaTypes.js";

test("getContract", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue({});
  const cometa = new CometaClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });

  await cometa.getContract("0x12345");

  expect(fn).toHaveBeenCalledOnce();
  expect(fn).toHaveBeenLastCalledWith({
    method: "cometa_getContract",
    params: ["0x12345"],
  });
});

test("getLocation", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue({});
  const cometa = new CometaClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });

  await cometa.getLocation("0x12345", 1);

  expect(fn).toHaveBeenCalledOnce();
  expect(fn).toHaveBeenLastCalledWith({
    method: "cometa_getLocation",
    params: ["0x12345", 1],
  });
});

test("compileContract", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue({});
  const cometa = new CometaClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });

  await cometa.compileContract("inputJson");

  expect(fn).toHaveBeenCalledOnce();
  expect(fn).toHaveBeenLastCalledWith({
    method: "cometa_compileContract",
    params: ["inputJson"],
  });
});

test("registerContractData", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue({});
  const cometa = new CometaClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });

  await cometa.registerContractData({} as ContractData, "0x12345");

  expect(fn).toHaveBeenCalledOnce();
  expect(fn).toHaveBeenLastCalledWith({
    method: "cometa_registerContractData",
    params: [{}, "0x12345"],
  });
});

test("registerContract", async ({ expect }) => {
  const fn = vi.fn();
  fn.mockReturnValue({});
  const cometa = new CometaClient({
    transport: new MockTransport(fn),
    shardId: 1,
  });

  await cometa.registerContract("input", "0x12345");

  expect(fn).toHaveBeenCalledOnce();
  expect(fn).toHaveBeenLastCalledWith({
    method: "cometa_registerContract",
    params: ["input", "0x12345"],
  });
});
