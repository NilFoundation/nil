import type { Abi } from "abitype";
import { expect } from "vitest";
import { generateTestSmartAccount, newClient } from "./helpers.js";

const client = newClient();

const abi: Abi = [
  {
    inputs: [{ internalType: "int32", name: "val", type: "int32" }],
    name: "add",
    outputs: [],
    stateMutability: "nonpayable",
    type: "function",
  },
  {
    inputs: [],
    name: "get",
    outputs: [{ internalType: "int32", name: "", type: "int32" }],
    stateMutability: "view",
    type: "function",
  },
  {
    inputs: [
      { internalType: "uint256", name: "", type: "uint256" },
      { internalType: "bytes", name: "", type: "bytes" },
    ],
    name: "verifyExternal",
    outputs: [{ internalType: "bool", name: "", type: "bool" }],
    stateMutability: "pure",
    type: "function",
  },
];

test("Call counter status", async () => {
  const smartAccount = await generateTestSmartAccount();

  const { address, tx } = await smartAccount.deployContract({
    shardId: 1,
    bytecode:
      "0x608060405234801561000f575f80fd5b506103808061001d5f395ff3fe608060405234801561000f575f80fd5b506004361061003f575f3560e01c806357b8a50f146100435780636d4ce63c1461005f578063796d7f561461007d575b5f80fd5b61005d6004803603810190610058919061014b565b6100ad565b005b6100676100ed565b6040516100749190610185565b60405180910390f35b61009760048036038101906100929190610232565b610101565b6040516100a491906102a9565b60405180910390f35b805f808282829054906101000a900460030b6100c991906102ef565b92506101000a81548163ffffffff021916908360030b63ffffffff16021790555050565b5f805f9054906101000a900460030b905090565b5f600190509392505050565b5f80fd5b5f80fd5b5f8160030b9050919050565b61012a81610115565b8114610134575f80fd5b50565b5f8135905061014581610121565b92915050565b5f602082840312156101605761015f61010d565b5b5f61016d84828501610137565b91505092915050565b61017f81610115565b82525050565b5f6020820190506101985f830184610176565b92915050565b5f819050919050565b6101b08161019e565b81146101ba575f80fd5b50565b5f813590506101cb816101a7565b92915050565b5f80fd5b5f80fd5b5f80fd5b5f8083601f8401126101f2576101f16101d1565b5b8235905067ffffffffffffffff81111561020f5761020e6101d5565b5b60208301915083600182028301111561022b5761022a6101d9565b5b9250929050565b5f805f604084860312156102495761024861010d565b5b5f610256868287016101bd565b935050602084013567ffffffffffffffff81111561027757610276610111565b5b610283868287016101dd565b92509250509250925092565b5f8115159050919050565b6102a38161028f565b82525050565b5f6020820190506102bc5f83018461029a565b92915050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52601160045260245ffd5b5f6102f982610115565b915061030483610115565b925082820190507fffffffffffffffffffffffffffffffffffffffffffffffffffffffff800000008112637fffffff82131715610344576103436102c2565b5b9291505056fea26469706673582212205d80a4424f46b63fd21864ea4f86d4e8c43cf3351e590d82c7c556c2664ebe1564736f6c63430008150033",
    salt: BigInt(Math.floor(Math.random() * 1000000000)),
    abi,
    args: [],
  });

  await tx.wait();

  const res = await client.call(
    {
      to: address,
      abi,
      functionName: "get",
    },
    "latest",
  );

  expect(res.decodedData).toEqual(0);

  const transactionHash = await smartAccount.sendTransaction({
    to: address,
    abi,
    functionName: "add",
    args: [100],
    value: 0n,
  });
  await transactionHash.wait();

  const resAfter = await client.call(
    {
      to: address,
      abi,
      functionName: "get",
    },
    "latest",
  );

  expect(resAfter.decodedData).toEqual(100);

  const syncTransactionHash = await smartAccount.syncSendTransaction({
    to: address,
    abi,
    functionName: "add",
    args: [100],
    value: 0n,
    gas: 1000000n,
    maxPriorityFeePerGas: 10n,
    maxFeePerGas: 1_000_000_000_000n,
  });

  const receipts = await syncTransactionHash.wait();

  const resAfterSync = await client.call(
    {
      to: address,
      abi,
      functionName: "get",
    },
    "latest",
  );

  expect(resAfterSync.decodedData).toEqual(200);
});
