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
import { combine, sample } from "effector";
import { persist as persistLocalStorage } from "effector-storage/local";
import { persist as persistSessionStorage } from "effector-storage/session";
import { loadedPlaygroundPage } from "../code/model";
import { sendMethodFx } from "../contracts/models/base";
import { playgroundRoute, playgroundWithHashRoute } from "../routing";
import { getRuntimeConfigOrThrow } from "../runtime-config";
import { nilAddress } from "../tokens";
import { $faucets } from "../tokens/model";
import { ActiveComponent } from "./ActiveComponent.ts";
import {
  $accountConnectorWithRpcUrl,
  $activeComponent,
  $balance,
  $balanceToken,
  $initializingSmartAccountError,
  $initializingSmartAccountState,
  $latestActivity,
  $privateKey,
  $rpcUrl,
  $smartAccount,
  $topUpError,
  $topupInput,
  addActivity,
  clearLatestActivity,
  createSmartAccountFx,
  defaultPrivateKey,
  fetchBalanceFx,
  fetchBalanceTokensFx,
  initializePrivateKey,
  initilizeSmartAccount,
  regenrateAccountEvent,
  resetTopUpError,
  setActiveComponent,
  setInitializingSmartAccountState,
  setPrivateKey,
  setRpcUrl,
  setTopupInput,
  topUpEvent,
  topUpSmartAccountBalanceFx,
  topupSmartAccountTokenFx,
  topupTokenEvent,
} from "./model";

persistLocalStorage({
  store: $rpcUrl,
  key: "rpcUrl",
});

persistLocalStorage({
  store: $privateKey,
  key: "privateKey",
});

$privateKey.on(setPrivateKey, (_, privateKey) => privateKey);
$rpcUrl.on(setRpcUrl, (_, rpcUrl) => rpcUrl);

createSmartAccountFx.use(async ({ privateKey, rpcUrl }) => {
  if (rpcUrl === "") {
    throw new Error("RPC URL should be provided");
  }

  const signer = new LocalECDSAKeySigner({ privateKey });
  const client = new PublicClient({
    transport: new HttpTransport({ endpoint: rpcUrl }),
  });
  const faucetClient = new FaucetClient({
    transport: new HttpTransport({ endpoint: rpcUrl }),
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
      return { smartAccount, rpcUrl };
    }

    await faucetClient.topUpAndWaitUntilCompletion(
      {
        smartAccountAddress: smartAccount.address,
        faucetAddress: faucets.NIL,
        amount: convertEthToWei(0.1),
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
    return { smartAccount, rpcUrl };
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
          amount: 10n,
        },
        client,
      );
    });

    await Promise.all(promises);
  }

  return { smartAccount, rpcUrl };
});

topUpSmartAccountBalanceFx.use(async (smartAccount) => {
  const faucetClient = new FaucetClient({
    transport: smartAccount.client.transport,
  });
  const faucets = await faucetClient.getAllFaucets();

  await faucetClient.topUpAndWaitUntilCompletion(
    {
      smartAccountAddress: smartAccount.address,
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

$smartAccount.reset($privateKey);
$smartAccount.on(createSmartAccountFx.doneData, (_, { smartAccount }) => smartAccount);

sample({
  clock: createSmartAccountFx.doneData,
  fn: ({ rpcUrl }) => rpcUrl,
  target: setRpcUrl,
});

sample({
  source: $privateKey,
  fn: (privateKey, rpcUrlEventPayload) => ({
    privateKey,
    rpcUrl: rpcUrlEventPayload,
  }),
  clock: initilizeSmartAccount,
  filter: (_, rpcUrl) => rpcUrl !== "",
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
  fn: ({ smartAccount }) => smartAccount,
  target: fetchBalanceFx,
});

sample({
  clock: createSmartAccountFx.doneData,
  fn: ({ smartAccount }) => smartAccount,
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

initilizeSmartAccount($rpcUrl.getState());

$activeComponent.on(setActiveComponent, (_, payload) => payload);

persistSessionStorage({
  store: $activeComponent,
  key: "activeComponentSmartAccount",
});

$topupInput.on(setTopupInput, (_, payload) => payload);

topupSmartAccountTokenFx.use(async ({ smartAccount, topupInput, faucets, rpcUrl }) => {
  const { token, amount } = topupInput;
  const faucetClient = new FaucetClient({
    transport: new HttpTransport({ endpoint: rpcUrl }),
  });

  const publicClient = new PublicClient({
    transport: new HttpTransport({
      endpoint: rpcUrl,
    }),
  });

  const tokenFaucetAddress = faucets[token];

  const txHash = await faucetClient.topUpAndWaitUntilCompletion(
    {
      smartAccountAddress: smartAccount.address,
      faucetAddress: tokenFaucetAddress,
      amount: tokenFaucetAddress === faucets.NIL ? convertEthToWei(Number(amount)) : BigInt(amount),
    },
    publicClient,
  );

  // Verify transaction receipt
  const receipt = await smartAccount.client.getTransactionReceiptByHash(txHash as Hex);
  if (!receipt?.success) {
    addActivity({ txHash, successful: false });
  }

  addActivity({ txHash, successful: true });
});

sample({
  clock: topupTokenEvent,
  source: combine(
    $smartAccount,
    $topupInput,
    $faucets,
    $rpcUrl,
    (smartAccount, topupInput, faucets, rpcUrl) =>
      ({
        smartAccount,
        topupInput,
        faucets,
        rpcUrl,
      }) as {
        smartAccount: SmartAccountV1;
        topupInput: { token: string; amount: string };
        faucets: Record<string, Hex>;
        rpcUrl: string;
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
  clock: loadedPlaygroundPage,
  source: combine(playgroundRoute.$query, playgroundWithHashRoute.$query, (query1, query2) => {
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
  target: setRpcUrl,
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
$initializingSmartAccountError.reset($accountConnectorWithRpcUrl);

$activeComponent.on(topupSmartAccountTokenFx.done, () => ActiveComponent.Main);

$topUpError
  .on(topupSmartAccountTokenFx.fail, () => "Top-up failed. Please try again")
  .on(resetTopUpError, () => "");

$initializingSmartAccountError
  .on(createSmartAccountFx.fail, () => "Failed to initialize smart account")
  .on(createSmartAccountFx, () => "");

let timeoutId: ReturnType<typeof setTimeout> | null = null;

$latestActivity.on(addActivity, (_, payload) => {
  // Clear any existing timeout
  if (timeoutId) clearTimeout(timeoutId);

  // Set new activity
  timeoutId = setTimeout(() => {
    clearLatestActivity();
    timeoutId = null;
  }, 10000);

  return payload;
});

$latestActivity.on(clearLatestActivity, () => null);
