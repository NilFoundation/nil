import type { Abi } from "abitype";
import {
  SmartAccountV1,
  bytesToHex,
  externalDeploymentTransaction,
  waitTillCompleted,
} from "../../src/index.js";
import { generatePublicKey, generateTestSmartAccount, newClient, topUpTest } from "./helpers.js";

const client = newClient();

test("Deploy through smart account", async ({ expect }) => {
  const smartAccount = await generateTestSmartAccount();

  const gasPrice = await client.getGasPrice(1);
  const pubKey = generatePublicKey();

  const abi = [
    {
      inputs: [
        { internalType: "bytes", name: "_pubkey", type: "bytes" },
        { internalType: "address", name: "_owner", type: "address" },
      ],
      stateMutability: "payable",
      type: "constructor",
    },
  ] as Abi;
  const { address, tx } = await smartAccount.deployContract({
    bytecode:
      "0x60806040526040516105a23803806105a283398181016040528101906100259190610222565b815f90816100339190610489565b508060015f6101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055505050610558565b5f604051905090565b5f80fd5b5f80fd5b5f80fd5b5f80fd5b5f601f19601f8301169050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b6100da82610094565b810181811067ffffffffffffffff821117156100f9576100f86100a4565b5b80604052505050565b5f61010b61007b565b905061011782826100d1565b919050565b5f67ffffffffffffffff821115610136576101356100a4565b5b61013f82610094565b9050602081019050919050565b8281835e5f83830152505050565b5f61016c6101678461011c565b610102565b90508281526020810184848401111561018857610187610090565b5b61019384828561014c565b509392505050565b5f82601f8301126101af576101ae61008c565b5b81516101bf84826020860161015a565b91505092915050565b5f73ffffffffffffffffffffffffffffffffffffffff82169050919050565b5f6101f1826101c8565b9050919050565b610201816101e7565b811461020b575f80fd5b50565b5f8151905061021c816101f8565b92915050565b5f806040838503121561023857610237610084565b5b5f83015167ffffffffffffffff81111561025557610254610088565b5b6102618582860161019b565b92505060206102728582860161020e565b9150509250929050565b5f81519050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52602260045260245ffd5b5f60028204905060018216806102ca57607f821691505b6020821081036102dd576102dc610286565b5b50919050565b5f819050815f5260205f209050919050565b5f6020601f8301049050919050565b5f82821b905092915050565b5f6008830261033f7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff82610304565b6103498683610304565b95508019841693508086168417925050509392505050565b5f819050919050565b5f819050919050565b5f61038d61038861038384610361565b61036a565b610361565b9050919050565b5f819050919050565b6103a683610373565b6103ba6103b282610394565b848454610310565b825550505050565b5f90565b6103ce6103c2565b6103d981848461039d565b505050565b5b818110156103fc576103f15f826103c6565b6001810190506103df565b5050565b601f82111561044157610412816102e3565b61041b846102f5565b8101602085101561042a578190505b61043e610436856102f5565b8301826103de565b50505b505050565b5f82821c905092915050565b5f6104615f1984600802610446565b1980831691505092915050565b5f6104798383610452565b9150826002028217905092915050565b6104928261027c565b67ffffffffffffffff8111156104ab576104aa6100a4565b5b6104b582546102b3565b6104c0828285610400565b5f60209050601f8311600181146104f1575f84156104df578287015190505b6104e9858261046e565b865550610550565b601f1984166104ff866102e3565b5f5b8281101561052657848901518255600182019150602085019450602081019050610501565b86831015610543578489015161053f601f891682610452565b8355505b6001600288020188555050505b505050505050565b603e806105645f395ff3fe60806040525f80fdfea2646970667358221220c97d85dd4ffadacb57164f781e1c8ef8477bb02fc13c0d0831811e05773b4a5164736f6c63430008190033",
    abi: abi,
    args: [pubKey, smartAccount.address],
    value: 10000000n,
    feeCredit: 1000000n * gasPrice,
    salt: 400n,
    shardId: 1,
  });

  const receipts = await tx.wait();

  expect(receipts.some((receipt) => !receipt.success)).toBe(false);

  const code = await client.getCode(address, "latest");

  expect(code).toBeDefined();
  expect(code.length).toBeGreaterThan(10);
});

test("External deployment", async ({ expect }) => {
  const chainId = await client.chainId();
  const gasPrice = await client.getGasPrice(1);

  const pubKey = generatePublicKey();
  const deploymentTransaction = externalDeploymentTransaction(
    {
      salt: 100n,
      shard: 1,
      bytecode: SmartAccountV1.code,
      abi: SmartAccountV1.abi,
      args: [pubKey],
      feeCredit: 1000000n * gasPrice,
    },
    chainId,
  );
  const addr = bytesToHex(deploymentTransaction.to);
  expect(addr).toBeDefined();

  await topUpTest(addr);

  const hash = await deploymentTransaction.send(client);

  const receipts = await waitTillCompleted(client, hash);

  expect(receipts.some((receipt) => !receipt.success)).toBe(false);

  const code = await client.getCode(addr, "latest");

  expect(code).toBeDefined();
  expect(code.length).toBeGreaterThan(10);
});

test("External failed deployment", async ({ expect }) => {
  const chainId = await client.chainId();
  const gasPrice = await client.getGasPrice(1);

  const pubKey = generatePublicKey();
  const deploymentTransaction = externalDeploymentTransaction(
    {
      salt: 100n,
      shard: 1,
      bytecode: SmartAccountV1.code,
      abi: SmartAccountV1.abi,
      args: [pubKey],
      feeCredit: 100000000n * gasPrice,
    },
    chainId,
  );
  const addr = bytesToHex(deploymentTransaction.to);
  expect(addr).toBeDefined();

  await topUpTest(addr, "NIL");

  const hash = await deploymentTransaction.send(client);

  const receipts = await waitTillCompleted(client, hash);

  expect(receipts.some((receipt) => !receipt.success)).toBe(true);
});
