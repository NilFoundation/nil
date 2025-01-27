import {
  FaucetClient,
  type Hex,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  SmartAccountV1,
  addHexPrefix,
  convertEthToWei,
  generateRandomPrivateKey,
  removeHexPrefix,
} from "@nilfoundation/niljs";
import { combine, forward, sample } from "effector";
import { persist as persistLocalStorage } from "effector-storage/local";
import { persist as persistSessionStorage } from "effector-storage/session";
import { loadedPage } from "../code/model";
import { sendMethodFx } from "../contracts/models/base";
import { sandboxRoute, sandboxWithHashRoute } from "../routing";
import { getRuntimeConfigOrThrow } from "../runtime-config";
import { nilAddress } from "../tokens";
import { $faucets } from "../tokens/model";
import {
  $accountConnectorWithEndpoint,
  $activeComponent,
  $balance,
  $balanceToken,
  $endpoint,
  $initializingSmartAccountError,
  $initializingSmartAccountState,
  $privateKey,
  $smartAccount,
  $topupInput,
  createSmartAccountFx,
  defaultPrivateKey,
  fetchBalanceFx,
  fetchBalanceTokensFx,
  initializePrivateKey,
  initilizeSmartAccount,
  regenrateAccountEvent,
  setActiveComponent,
  setEndpoint,
  setInitializingSmartAccountState,
  setPrivateKey,
  setTopupInput,
  topUpEvent,
  topUpSmartAccountBalanceFx,
  topupSmartAccountTokenFx,
  topupTokenEvent,
} from "./model";

persistLocalStorage({
  store: $endpoint,
  key: "endpoint",
});

persistLocalStorage({
  store: $privateKey,
  key: "privateKey",
});

$privateKey.on(setPrivateKey, (_, privateKey) => privateKey);
$endpoint.on(setEndpoint, (_, endpoint) => endpoint);

createSmartAccountFx.use(async ({ privateKey, endpoint }) => {
  const signer = new LocalECDSAKeySigner({ privateKey });
  const client = new PublicClient({
    transport: new HttpTransport({ endpoint }),
  });
  const faucetClient = new FaucetClient({
    transport: new HttpTransport({ endpoint }),
  });
  const faucets = await faucetClient.getAllFaucets();

  const pubkey = signer.getPublicKey();
  const smartAccount = new SmartAccountV1({
    pubkey,
    salt: 100n,
    shardId: 1,
    client,
    signer,
  });

  setInitializingSmartAccountState("Checking balance...");

  const balance = await smartAccount.getBalance();

  if (balance === 0n) {
    if (!faucets) {
      throw new Error("No faucets available");
    }

    await faucetClient.topUpAndWaitUntilCompletion(
      {
        smartAccountAddress: smartAccount.address,
        faucetAddress: faucets.NIL,
        amount: 1e18,
      },
      client,
    );
  }

  setInitializingSmartAccountState("Checking if smart account is deployed...");

  const code = await client.getCode(smartAccount.address);
  if (code.length === 0) {
    await smartAccount.selfDeploy(true);
  }

  setInitializingSmartAccountState("Adding some tokens...");

  if (!faucets) {
    return smartAccount;
  }

  const tokensMap = await smartAccount.client.getTokens(smartAccount.address, "latest");

  const tokens = Object.entries(tokensMap).map(([token]) =>
    addHexPrefix(removeHexPrefix(token).padStart(40, "0")),
  );
  const tokensWithZeroBalance = Object.values(faucets).filter(
    (addr) => !tokens.some((token) => token === addr || token !== nilAddress),
  );

  if (tokensWithZeroBalance.length > 0) {
    const promises = tokensWithZeroBalance.map((token) => {
      const tokenFaucetAddress = Object.values(faucets).find((addr) => addr === token);

      if (!tokenFaucetAddress) {
        return Promise.resolve();
      }

      return faucetClient.topUpAndWaitUntilCompletion(
        {
          smartAccountAddress: smartAccount.address,
          faucetAddress: tokenFaucetAddress,
          amount: 10,
        },
        client,
      );
    });

    await Promise.all(promises);
  }

  return smartAccount;
});

topUpSmartAccountBalanceFx.use(async (smartAccount) => {
  const faucetClient = new FaucetClient({
    transport: smartAccount.client.transport,
  });
  const faucets = await faucetClient.getAllFaucets();

  await faucetClient.topUpAndWaitUntilCompletion(
    {
      smartAccount: smartAccount.address,
      faucetAddress: faucets.NIL,
      amount: convertEthToWei(0.1),
    },
    smartAccount.client,
  );
  return await smartAccount.getBalance();
});

fetchBalanceFx.use(async (smartAccount) => {
  return await smartAccount.getBalance();
});

fetchBalanceTokensFx.use(async (smartAccount) => {
  return await smartAccount.client.getTokens(smartAccount.address, "latest");
});

createSmartAccountFx.failData.watch((error) => {
  console.error(error);
});

forward({
  from: combine($privateKey, $endpoint, $faucets, (privateKey, endpoint, faucets) => ({
    privateKey,
    endpoint,
    faucets,
  })),
  to: createSmartAccountFx,
});

$smartAccount.reset($privateKey);
$smartAccount.on(createSmartAccountFx.doneData, (_, smartAccount) => smartAccount);

sample({
  source: combine($privateKey, $endpoint, $faucets, (privateKey, endpoint, faucets) => ({
    privateKey,
    endpoint,
    faucets,
  })),
  clock: initilizeSmartAccount,
  target: createSmartAccountFx,
});

sample({
  clock: initializePrivateKey,
  filter: $privateKey.map((privateKey) => privateKey === defaultPrivateKey),
  fn: () => generateRandomPrivateKey(),
  target: setPrivateKey,
});

sample({
  clock: regenrateAccountEvent,
  fn: () => generateRandomPrivateKey(),
  target: setPrivateKey,
});

sample({
  clock: createSmartAccountFx.doneData,
  target: fetchBalanceFx,
});

sample({
  clock: createSmartAccountFx.doneData,
  target: fetchBalanceTokensFx,
});

sample({
  clock: topUpEvent,
  target: topUpSmartAccountBalanceFx,
  source: $smartAccount,
  filter: (smartAccount) => smartAccount !== null,
  fn: (smartAccount) => smartAccount as SmartAccountV1,
});

$balance.on(fetchBalanceFx.doneData, (_, balance) => balance);
$balance.on(topUpSmartAccountBalanceFx.doneData, (_, balance) => balance);
$balance.reset($smartAccount);

$balanceToken.on(fetchBalanceTokensFx.doneData, (_, tokens) => tokens);
$balanceToken.reset($smartAccount);

initializePrivateKey();

initilizeSmartAccount();

sample({
  clock: sendMethodFx.doneData,
  target: fetchBalanceFx,
  source: $smartAccount,
  filter: (smartAccount) => smartAccount !== null,
  fn: (smartAccount) => smartAccount as SmartAccountV1,
});

$activeComponent.on(setActiveComponent, (_, payload) => payload);

persistSessionStorage({
  store: $activeComponent,
  key: "activeComponentSmartAccount",
});

$topupInput.on(setTopupInput, (_, payload) => payload);

topupSmartAccountTokenFx.use(async ({ smartAccount, topupInput, faucets, endpoint }) => {
  const { token, amount } = topupInput;
  const faucetClient = new FaucetClient({
    transport: new HttpTransport({ endpoint }),
  });

  const publicClient = new PublicClient({
    transport: new HttpTransport({
      endpoint,
    }),
  });

  const tokenFaucetAddress = faucets[token];

  await faucetClient.topUpAndWaitUntilCompletion(
    {
      smartAccountAddress: smartAccount.address,
      faucetAddress: tokenFaucetAddress,
      amount: Number(amount),
    },
    publicClient,
  );
});

sample({
  clock: topupTokenEvent,
  source: combine(
    $smartAccount,
    $topupInput,
    $faucets,
    $endpoint,
    (smartAccount, topupInput, faucets, endpoint) =>
      ({
        smartAccount,
        topupInput,
        faucets,
        endpoint,
      }) as {
        smartAccount: SmartAccountV1;
        topupInput: { token: string; amount: string };
        faucets: Record<string, Hex>;
        endpoint: string;
      },
  ),
  target: topupSmartAccountTokenFx,
});

sample({
  clock: topupSmartAccountTokenFx.doneData,
  target: fetchBalanceTokensFx,
  source: $smartAccount,
  fn: (smartAccount) => smartAccount as SmartAccountV1,
  filter: (smartAccount) => smartAccount !== null,
});

sample({
  clock: topupSmartAccountTokenFx.doneData,
  target: fetchBalanceFx,
  source: $smartAccount,
  fn: (smartAccount) => smartAccount as SmartAccountV1,
  filter: (smartAccount) => smartAccount !== null,
});

sample({
  clock: loadedPage,
  source: combine(sandboxRoute.$query, sandboxWithHashRoute.$query, (query1, query2) => {
    const user = query1.user ?? query2.user;
    const token = query1.token ?? query2.token;
    return { user, token };
  }),
  fn: (q) => {
    const user = q.user;
    const token = q.token;
    return `${getRuntimeConfigOrThrow().RPC_API_URL}/${user}/${token}`;
  },
  filter: (q) => !!q.user && !!q.token,
  target: setEndpoint,
});

sample({
  clock: sendMethodFx.doneData,
  source: $smartAccount,
  fn: (smartAccount) => smartAccount as SmartAccountV1,
  filter: (smartAccount) => smartAccount !== null,
  target: [fetchBalanceFx, fetchBalanceTokensFx],
});

$initializingSmartAccountState.on(setInitializingSmartAccountState, (_, payload) => payload);
$initializingSmartAccountState.reset(createSmartAccountFx.done);

$initializingSmartAccountError.reset(createSmartAccountFx.done);
$initializingSmartAccountError.reset($accountConnectorWithEndpoint);

$initializingSmartAccountError.on(
  createSmartAccountFx.fail,
  () => "Failed to initialize smart account",
);
