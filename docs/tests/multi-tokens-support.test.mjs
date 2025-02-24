import { FAUCET_GLOBAL, NIL_GLOBAL, RPC_GLOBAL } from "./globals";

import {
  HttpTransport,
  PublicClient,
  generateSmartAccount,
  waitTillCompleted,
} from "@nilfoundation/niljs";

import { FT_COMPILATION_COMMAND } from "./compilationCommands";

import TestHelper from "./TestHelper";

import { CREATED_TOKEN_PATTERN, SMART_ACCOUNT_ADDRESS_PATTERN, TOKEN_PATTERN } from "./patterns";

const util = require("node:util");
const exec = util.promisify(require("node:child_process").exec);

const RPC_ENDPOINT = RPC_GLOBAL;
const FAUCET_ENDPOINT = FAUCET_GLOBAL;
const CONFIG_FILE_NAME = "./tests/tempConfigTokensMCCSupport.ini";

const NAME = "newToken";
const SALT = BigInt(Math.floor(Math.random() * 10000));

const AMOUNT = 5000;

const CONFIG_FLAG = `--config ${CONFIG_FILE_NAME}`;

const TOKENS_COMMAND = `${NIL_GLOBAL} contract tokens ${CONFIG_FLAG}`;

let TEST_COMMANDS;
let OWNER_ADDRESS;

beforeAll(async () => {
  const testHelper = new TestHelper({ configFileName: CONFIG_FILE_NAME });
  TEST_COMMANDS = testHelper.createCLICommandsMap(SALT);

  await exec(TEST_COMMANDS.KEYGEN_COMMAND);
  await exec(TEST_COMMANDS.RPC_COMMAND);
  await exec(TEST_COMMANDS.FAUCET_COMMAND);
  const { stdout, stderr } = await exec(TEST_COMMANDS.SMART_ACCOUNT_CREATION_COMMAND);
  OWNER_ADDRESS = stdout.match(SMART_ACCOUNT_ADDRESS_PATTERN)[0];
}, 20000);

afterAll(async () => {
  await exec(`rm -rf ${CONFIG_FILE_NAME}`);
});

describe.sequential("FT contract tests", () => {
  test.sequential("The FT contract is compiled", async () => {
    const { stdout, stderr } = await exec(FT_COMPILATION_COMMAND);
    expect(stdout).toBeDefined();
  });
});

describe.skip.sequential("initial usage CLI tests", () => {
  test.sequential("CLI creates a token and withdraws it", async () => {
    //startBasicCreateTokenCommand
    const CREATE_TOKEN_COMMAND = `${NIL_GLOBAL} minter create-token ${OWNER_ADDRESS} ${AMOUNT} ${NAME} ${CONFIG_FLAG}`;
    //endBasicCreateTokenCommand
    //startBasicWithdrawTokenCommand
    const BASIC_WITHDRAW_TOKEN_COMMAND = `${NIL_GLOBAL} minter withdraw-token ${OWNER_ADDRESS} ${AMOUNT} ${OWNER_ADDRESS} ${CONFIG_FLAG}`;
    //endBasicWithdrawTokenCommand
    let { stdout, stderr } = await exec(CREATE_TOKEN_COMMAND);
    expect(stdout).toMatch(CREATED_TOKEN_PATTERN);
    await exec(BASIC_WITHDRAW_TOKEN_COMMAND);
    const TOKENS_COMMAND_OWNER = `${TOKENS_COMMAND} ${OWNER_ADDRESS} ${CONFIG_FLAG}`;
    ({ stdout, stderr } = await exec(TOKENS_COMMAND_OWNER));
    expect(stdout).toBeDefined();
    expect(stdout).toMatch(TOKEN_PATTERN);
  });

  test.sequential("CLI mints an existing token", async () => {
    //startMintExistingTokenCommand
    const MINT_EXISTING_TOKEN_COMMAND = `${NIL_GLOBAL} minter mint-token ${OWNER_ADDRESS} 50000 ${CONFIG_FLAG}`;
    //endMintExistingTokenCommand
    let { stdout, stderr } = await exec(MINT_EXISTING_TOKEN_COMMAND);
    expect(stdout).toBeDefined();
    ({ stdout, stderr } = await exec(
      `${NIL_GLOBAL} contract tokens ${OWNER_ADDRESS} ${CONFIG_FLAG}`,
    ));
    expect(stdout).toBeDefined();
    expect(stdout).toMatch(/55000/);
  });

  test.sequential("CLI burns an existing token", async () => {
    const AMOUNT = 50000;
    //startBurnExistingTokenCommand
    const BURN_EXISTING_TOKEN_COMMAND = `${NIL_GLOBAL} minter burn-token ${OWNER_ADDRESS} ${AMOUNT} ${CONFIG_FLAG}`;
    //endBurnExistingTokenCommand
    let { stdout, stderr } = await exec(BURN_EXISTING_TOKEN_COMMAND);
    expect(stdout).toBeDefined();
    ({ stdout, stderr } = await exec(
      `${NIL_GLOBAL} contract tokens ${OWNER_ADDRESS} ${CONFIG_FLAG}`,
    ));
    expect(stdout).toBeDefined();
    expect(stdout).toMatch(TOKEN_PATTERN);
  });
});
describe.sequential("basic Nil.js usage tests", async () => {
  test.sequential(
    "Nil.js can create a new token, mint it, withdraw it, and burn it",
    async () => {
      //startBasicNilJSExample
      const client = new PublicClient({
        transport: new HttpTransport({
          endpoint: RPC_ENDPOINT,
        }),
        shardId: 1,
      });

      const smartAccount = await generateSmartAccount({
        shardId: 1,
        rpcEndpoint: RPC_ENDPOINT,
        faucetEndpoint: FAUCET_ENDPOINT,
      });

      {
        const hashTransaction = await smartAccount.setTokenName("MY_TOKEN");
        await waitTillCompleted(client, hashTransaction);
      }

      {
        const hashTransaction = await smartAccount.mintToken(100_000_000n);
        await waitTillCompleted(client, hashTransaction);
      }
      //endBasicNilJSExample

      //startNilJSBurningExample
      {
        const hashTransaction = await smartAccount.burnToken(50_000_000n);
        await waitTillCompleted(client, hashTransaction);
      }
      //endNilJSBurningExample

      const tokens = await client.getTokens(smartAccount.address, "latest");

      expect(Object.keys(tokens).length === 1);
      expect(Object.values(tokens)[0] === 50_000_000n);
    },
    80000,
  );
});

describe.sequential("tutorial flows Nil.js tests", async () => {
  test("Nil.js successfully creates two smart accounts and handles token transfers", async () => {
    //startAdvancedNilJSExample
    const client = new PublicClient({
      transport: new HttpTransport({
        endpoint: RPC_ENDPOINT,
      }),
      shardId: 1,
    });

    const smartAccount = await generateSmartAccount({
      shardId: 1,
      rpcEndpoint: RPC_ENDPOINT,
      faucetEndpoint: FAUCET_ENDPOINT,
    });

    const smartAccountTwo = await generateSmartAccount({
      shardId: 1,
      rpcEndpoint: RPC_ENDPOINT,
      faucetEndpoint: FAUCET_ENDPOINT,
    });

    {
      const hashTransaction = await smartAccount.setTokenName("MY_TOKEN");
      await waitTillCompleted(client, hashTransaction);
    }

    {
      const hashTransaction = await smartAccountTwo.setTokenName("ANOTHER_TOKEN");
      await waitTillCompleted(client, hashTransaction);
    }
    //endAdvancedNilJSExample

    //startAdvancedNilJSMintingExample
    {
      const hashTransaction = await smartAccount.mintToken(100_000_000n);
      await waitTillCompleted(client, hashTransaction);
    }

    {
      const hashTransaction = await smartAccountTwo.mintToken(50_000_000n);
      await waitTillCompleted(client, hashTransaction);
    }
    //endAdvancedNilJSMintingExample

    //startNilJSTransferExample
    const transferTransaction = smartAccountTwo.sendTransaction({
      to: smartAccount.address,
      value: 1_000_000n,
      feeCredit: 5_000_000n,
      tokens: [
        {
          id: smartAccountTwo.address,
          amount: 50_000_000n,
        },
      ],
    });
    const tokens = await client.getTokens(smartAccount.address, "latest");
    //endNilJSTransferExample

    expect(Object.keys(tokens).length === 2);
  }, 80000);
});
