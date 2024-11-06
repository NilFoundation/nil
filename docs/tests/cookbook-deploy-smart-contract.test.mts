import { RPC_GLOBAL } from "./globals";

//startImportStatements
import {
  ExternalMessageEnvelope,
  Faucet,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
  bytesToHex,
  convertEthToWei,
  externalDeploymentMessage,
  generateRandomPrivateKey,
  hexToBytes,
  waitTillCompleted,
} from "@nilfoundation/niljs";

import { encodeFunctionData, type Abi } from "viem";
//endImportStatements

const RPC_ENDPOINT = RPC_GLOBAL;

let COUNTER_ADDRESS: `0x${string}`;

const COUNTER_BYTECODE =
  "0x608060405234801561000f575f80fd5b506103ee8061001d5f395ff3fe608060405260043610610037575f3560e01c80632096525514610042578063796d7f561461006c578063d09de08a146100a85761003e565b3661003e57005b5f80fd5b34801561004d575f80fd5b506100566100be565b604051610063919061013b565b60405180910390f35b348015610077575f80fd5b50610092600480360381019061008d91906102cb565b6100c6565b60405161009f919061033f565b60405180910390f35b3480156100b3575f80fd5b506100bc6100d1565b005b5f8054905090565b5f6001905092915050565b60015f808282546100e29190610385565b925050819055507f93fe6d397c74fdf1402a8b72e47b68512f0510d7b98a4bc4cbdf6ac7108b3c595f54604051610119919061013b565b60405180910390a1565b5f819050919050565b61013581610123565b82525050565b5f60208201905061014e5f83018461012c565b92915050565b5f604051905090565b5f80fd5b5f80fd5b61016e81610123565b8114610178575f80fd5b50565b5f8135905061018981610165565b92915050565b5f80fd5b5f80fd5b5f601f19601f8301169050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b6101dd82610197565b810181811067ffffffffffffffff821117156101fc576101fb6101a7565b5b80604052505050565b5f61020e610154565b905061021a82826101d4565b919050565b5f67ffffffffffffffff821115610239576102386101a7565b5b61024282610197565b9050602081019050919050565b828183375f83830152505050565b5f61026f61026a8461021f565b610205565b90508281526020810184848401111561028b5761028a610193565b5b61029684828561024f565b509392505050565b5f82601f8301126102b2576102b161018f565b5b81356102c284826020860161025d565b91505092915050565b5f80604083850312156102e1576102e061015d565b5b5f6102ee8582860161017b565b925050602083013567ffffffffffffffff81111561030f5761030e610161565b5b61031b8582860161029e565b9150509250929050565b5f8115159050919050565b61033981610325565b82525050565b5f6020820190506103525f830184610330565b92915050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52601160045260245ffd5b5f61038f82610123565b915061039a83610123565b92508282019050808211156103b2576103b1610358565b5b9291505056fea26469706673582212207a028b9449dd0d8aeb30c0fef2da7fce14fd80ee5a5edfbba2590caf807387bf64736f6c63430008150033";

const COUNTER_ABI = [
  {
    anonymous: false,
    inputs: [{ indexed: false, internalType: "uint256", name: "newValue", type: "uint256" }],
    name: "ValueChanged",
    type: "event",
  },
  {
    inputs: [],
    name: "getValue",
    outputs: [{ internalType: "uint256", name: "", type: "uint256" }],
    stateMutability: "view",
    type: "function",
  },
  { inputs: [], name: "increment", outputs: [], stateMutability: "nonpayable", type: "function" },
  {
    inputs: [
      { internalType: "uint256", name: "hash", type: "uint256" },
      { internalType: "bytes", name: "authData", type: "bytes" },
    ],
    name: "verifyExternal",
    outputs: [{ internalType: "bool", name: "", type: "bool" }],
    stateMutability: "pure",
    type: "function",
  },
  { stateMutability: "payable", type: "receive" },
];

describe.sequential("Nil.js passes the deployment and calling flow", async () => {
  test.sequential(
    "Nil.js can deploy Counter internally",
    async () => {
      //startInternalDeployment
      const SALT = BigInt(Math.floor(Math.random() * 10000));

      const client = new PublicClient({
        transport: new HttpTransport({
          endpoint: RPC_ENDPOINT,
        }),
        shardId: 1,
      });

      const faucet = new Faucet(client);

      const pkey = generateRandomPrivateKey();

      const signer = new LocalECDSAKeySigner({
        privateKey: pkey,
      });

      const pubkey = signer.getPublicKey();

      const wallet = new WalletV1({
        pubkey: pubkey,
        client: client,
        signer: signer,
        shardId: 1,
        salt: SALT,
      });

      const walletAddress = wallet.address;

      await faucet.withdrawToWithRetry(walletAddress, convertEthToWei(10));

      await wallet.selfDeploy(true);

      const { address, hash } = await wallet.deployContract({
        bytecode: COUNTER_BYTECODE,
        abi: COUNTER_ABI as unknown as Abi,
        args: [],
        feeCredit: 10_000_000n,
        salt: SALT,
        shardId: 1,
      });

      const manufacturerReceipts = await waitTillCompleted(client, 1, hash);
      //endInternalDeployment

      COUNTER_ADDRESS = address;

      expect(manufacturerReceipts.some((receipt) => !receipt.success)).toBe(false);

      const code = await client.getCode(address, "latest");

      expect(code).toBeDefined();
      expect(code.length).toBeGreaterThan(10);
    },
    40000,
  );

  test.sequential("Nil.js can deploy Counter externally", async () => {
    //startExternalDeployment
    const SALT = BigInt(Math.floor(Math.random() * 10000));

    const client = new PublicClient({
      transport: new HttpTransport({
        endpoint: RPC_ENDPOINT,
      }),
      shardId: 1,
    });

    const faucet = new Faucet(client);

    const chainId = await client.chainId();

    const deploymentMessage = externalDeploymentMessage(
      {
        salt: SALT,
        shard: 1,
        bytecode: COUNTER_BYTECODE,
        abi: COUNTER_ABI as unknown as Abi,
        args: [],
      },
      chainId,
    );

    const addr = bytesToHex(deploymentMessage.to);

    const faucetHash = await faucet.withdrawToWithRetry(addr, convertEthToWei(0.1));

    await waitTillCompleted(client, 1, faucetHash);

    const hash = await deploymentMessage.send(client);

    const receipts = await waitTillCompleted(client, 1, hash);
    //endExternalDeployment

    expect(receipts.some((receipt) => !receipt.success)).toBe(false);

    const code = await client.getCode(addr, "latest");

    expect(code).toBeDefined();
    expect(code.length).toBeGreaterThan(10);
  });

  test.sequential("Nil.js can call Counter successfully with an internal message", async () => {
    const SALT = BigInt(Math.floor(Math.random() * 10000));

    const client = new PublicClient({
      transport: new HttpTransport({
        endpoint: RPC_ENDPOINT,
      }),
      shardId: 1,
    });

    const faucet = new Faucet(client);

    const pkey = generateRandomPrivateKey();

    const signer = new LocalECDSAKeySigner({
      privateKey: pkey,
    });

    const pubkey = signer.getPublicKey();

    const wallet = new WalletV1({
      pubkey: pubkey,
      client: client,
      signer: signer,
      shardId: 1,
      salt: SALT,
    });

    const walletAddress = wallet.address;

    await faucet.withdrawToWithRetry(walletAddress, convertEthToWei(10));

    await wallet.selfDeploy(true);

    //startInternalMessage
    const hash = await wallet.sendMessage({
      to: COUNTER_ADDRESS,
      abi: COUNTER_ABI as unknown as Abi,
      feeCredit: 5_000_000n,
      functionName: "increment",
    });

    const receipts = await waitTillCompleted(client, 1, hash);
    //endInternalMessage

    expect(receipts.some((receipt) => !receipt.success)).toBe(false);
  });

  test.sequential(
    "Nil.js can call Counter successfully with an external message",
    async () => {
      const client = new PublicClient({
        transport: new HttpTransport({
          endpoint: RPC_ENDPOINT,
        }),
        shardId: 1,
      });

      const faucet = new Faucet(client);

      await faucet.withdrawToWithRetry(COUNTER_ADDRESS, convertEthToWei(10));

      const chainId = await client.chainId();
      //startExternalMessage
      const message = new ExternalMessageEnvelope({
        to: hexToBytes(COUNTER_ADDRESS),
        isDeploy: false,
        chainId,
        data: hexToBytes(
          encodeFunctionData({
            abi: COUNTER_ABI as unknown as Abi,
            functionName: "increment",
            args: [],
          }),
        ),
        authData: new Uint8Array(0),
        seqno: await client.getMessageCount(COUNTER_ADDRESS),
      });

      const encodedMessage = message.encode();

      let success = false;
      let messageHash: `0x${string}`;

      while (!success) {
        try {
          messageHash = await client.sendRawMessage(bytesToHex(encodedMessage));
          success = true;
        } catch (error) {
          await new Promise((resolve) => setTimeout(resolve, 1000));
        }
      }

      const receipts = await waitTillCompleted(client, 1, messageHash);
      //endExternalMessage

      expect(receipts.some((receipt) => !receipt.success)).toBe(false);
    },
    40000,
  );
});
