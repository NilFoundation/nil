import { RPC_GLOBAL, NIL_GLOBAL } from "./globals";

import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const {
  Faucet,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
  generateRandomPrivateKey,
  convertEthToWei,
  waitTillCompleted,
  bytesToHex,
  FaucetClient,
} = require("@nilfoundation/niljs");

import TestHelper from "./TestHelper";

import { WALLET_ADDRESS_PATTERN, CREATED_CURRENCY_PATTERN, CURRENCY_PATTERN } from "./patterns";

const util = require("node:util");
const exec = util.promisify(require("node:child_process").exec);

const RPC_ENDPOINT = RPC_GLOBAL;
const CONFIG_FILE_NAME = "./tests/tempConfigTokensMCCSupport.ini";

const NAME = "newToken";
const SALT = BigInt(Math.floor(Math.random() * 10000));

const AMOUNT = 5000;

const CONFIG_FLAG = `--config ${CONFIG_FILE_NAME}`;

const CURRENCIES_COMMAND = `${NIL_GLOBAL} contract currencies ${CONFIG_FLAG}`;

let TEST_COMMANDS;
let OWNER_ADDRESS;

beforeAll(async () => {
  const testHelper = new TestHelper({ configFileName: CONFIG_FILE_NAME });
  TEST_COMMANDS = testHelper.createCLICommandsMap(SALT);

  await exec(TEST_COMMANDS["KEYGEN_COMMAND"]);
  await exec(TEST_COMMANDS["RPC_COMMAND"]);
  const { stdout, stderr } = await exec(TEST_COMMANDS["WALLET_CREATION_COMMAND"]);
  OWNER_ADDRESS = stdout.match(WALLET_ADDRESS_PATTERN)[0];
}, 20000);

afterAll(async () => {
  await exec(`rm -rf ${CONFIG_FILE_NAME}`);
});

describe.skip.sequential("initial usage CLI tests", () => {
  test.sequential("CLI creates a currency and withdraws it", async () => {
    //startBasicCreateCurrencyCommand
    const CREATE_CURRENCY_COMMAND = `${NIL_GLOBAL} minter create-currency ${OWNER_ADDRESS} ${AMOUNT} ${NAME} ${CONFIG_FLAG}`;
    //endBasicCreateCurrencyCommand
    //startBasicWithdrawCurrencyCommand
    const BASIC_WITHDRAW_CURRENCY_COMMAND = `${NIL_GLOBAL} minter withdraw-currency ${OWNER_ADDRESS} ${AMOUNT} ${OWNER_ADDRESS} ${CONFIG_FLAG}`;
    //endBasicWithdrawCurrencyCommand
    let { stdout, stderr } = await exec(CREATE_CURRENCY_COMMAND);
    expect(stdout).toMatch(CREATED_CURRENCY_PATTERN);
    await exec(BASIC_WITHDRAW_CURRENCY_COMMAND);
    const CURRENCIES_COMMAND_OWNER = `${CURRENCIES_COMMAND} ${OWNER_ADDRESS} ${CONFIG_FLAG}`;
    ({ stdout, stderr } = await exec(CURRENCIES_COMMAND_OWNER));
    expect(stdout).toBeDefined();
    expect(stdout).toMatch(CURRENCY_PATTERN);
  });

  test.sequential("CLI mints an existing currency", async () => {
    //startMintExistingCurrencyCommand
    const MINT_EXISTING_CURRENCY_COMMAND = `${NIL_GLOBAL} minter mint-currency ${OWNER_ADDRESS} 50000 ${CONFIG_FLAG}`;
    //endMintExistingCurrencyCommand
    let { stdout, stderr } = await exec(MINT_EXISTING_CURRENCY_COMMAND);
    expect(stdout).toBeDefined();
    ({ stdout, stderr } = await exec(
      `${NIL_GLOBAL} contract currencies ${OWNER_ADDRESS} ${CONFIG_FLAG}`,
    ));
    expect(stdout).toBeDefined();
    expect(stdout).toMatch(/55000/);
  });

  test.sequential("CLI burns an existing currency", async () => {
    const AMOUNT = 50000;
    //startBurnExistingCurrencyCommand
    const BURN_EXISTING_CURRENCY_COMMAND = `${NIL_GLOBAL} minter burn-currency ${OWNER_ADDRESS} ${AMOUNT} ${CONFIG_FLAG}`;
    //endBurnExistingCurrencyCommand
    let { stdout, stderr } = await exec(BURN_EXISTING_CURRENCY_COMMAND);
    expect(stdout).toBeDefined();
    ({ stdout, stderr } = await exec(
      `${NIL_GLOBAL} contract currencies ${OWNER_ADDRESS} ${CONFIG_FLAG}`,
    ));
    expect(stdout).toBeDefined();
    expect(stdout).toMatch(CURRENCY_PATTERN);
  });
});
describe.sequential("basic Nil.js usage tests", async () => {
  test.sequential(
    "Nil.js can create a new currency, mint it, withdraw it, and burn it",
    async () => {
      //startBasicNilJSExample
      const client = new PublicClient({
        transport: new HttpTransport({
          endpoint: RPC_ENDPOINT,
        }),
        shardId: 1,
      });

      const faucet = new Faucet(client);

      const signer = new LocalECDSAKeySigner({
        privateKey: generateRandomPrivateKey(),
      });

      const pubkey = signer.getPublicKey();
      const wallet = new WalletV1({
        pubkey: pubkey,
        salt: BigInt(Math.floor(Math.random() * 10000)),
        shardId: 1,
        client,
        signer,
      });

      const walletAddress = wallet.address;

      const faucetHash = await faucet.withdrawToWithRetry(walletAddress, convertEthToWei(1));

      await wallet.selfDeploy(true);

      {
        const hashMessage = await wallet.setCurrencyName("MY_TOKEN");
        await waitTillCompleted(client, 1, hashMessage);
      }

      {
        const hashMessage = await wallet.mintCurrency(100_000_000n);
        await waitTillCompleted(client, 1, hashMessage);
      }
      //endBasicNilJSExample

      //startNilJSBurningExample
      {
        const hashMessage = await wallet.burnCurrency(50_000_000n);
        await waitTillCompleted(client, 1, hashMessage);
      }
      //endNilJSBurningExample

      const tokens = await client.getCurrencies(walletAddress, "latest");

      expect(Object.keys(tokens).length === 1);
      expect(Object.values(tokens)[0] === 50_000_000n);
    },
    80000,
  );
});

describe.sequential("tutorial flows Nil.js tests", async () => {
  test("Nil.js successfully creates two wallets and handles currency transfers", async () => {
    //startAdvancedNilJSExample
    const client = new PublicClient({
      transport: new HttpTransport({
        endpoint: RPC_ENDPOINT,
      }),
      shardId: 1,
    });

    const faucet = new Faucet(client);

    const signer = new LocalECDSAKeySigner({
      privateKey: generateRandomPrivateKey(),
    });

    const pubkey = signer.getPublicKey();

    const wallet = new WalletV1({
      pubkey: pubkey,
      salt: BigInt(Math.floor(Math.random() * 10000)),
      shardId: 1,
      client,
      signer,
    });

    const walletAddress = wallet.address;

    const faucetHash = await faucet.withdrawToWithRetry(walletAddress, convertEthToWei(1));

    await waitTillCompleted(client, 1, faucetHash);

    await wallet.selfDeploy(true);

    const walletTwo = new WalletV1({
      pubkey: pubkey,
      salt: BigInt(Math.floor(Math.random() * 10000)),
      shardId: 1,
      client,
      signer,
    });

    const walletTwoAddress = walletTwo.address;

    const faucetTwoHash = await faucet.withdrawToWithRetry(walletTwoAddress, convertEthToWei(1));

    await walletTwo.selfDeploy(true);

    {
      const hashMessage = await wallet.setCurrencyName("MY_TOKEN");
      await waitTillCompleted(client, 1, hashMessage);
    }

    {
      const hashMessage = await walletTwo.setCurrencyName("ANOTHER_TOKEN");
      await waitTillCompleted(client, 1, hashMessage);
    }
    //endAdvancedNilJSExample

    //startAdvancedNilJSMintingExample
    {
      const hashMessage = await wallet.mintCurrency(100_000_000n);
      await waitTillCompleted(client, 1, hashMessage);
    }

    {
      const hashMessage = await wallet.mintCurrency(50_000_000n);
      await waitTillCompleted(client, 1, hashMessage);
    }
    //endAdvancedNilJSMintingExample

    //startNilJSTransferExample
    const transferMessage = walletTwo.sendMessage({
      to: walletAddress,
      value: 1_000_000n,
      feeCredit: 5_000_000n,
      tokens: [
        {
          id: walletTwoAddress,
          amount: 50_000_000n,
        },
      ],
    });
    const tokens = await client.getCurrencies(walletAddress, "latest");
    //endNilJSTransferExample

    expect(Object.keys(tokens).length === 2);
  }, 80000);
});
