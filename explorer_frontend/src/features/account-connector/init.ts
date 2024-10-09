import { forward, sample } from "effector";
import {
  setPrivateKey,
  $privateKey,
  $endpoint,
  setEndpoint,
  createWalletFx,
  $wallet,
  $accountConnectorWithEndpoint,
  initilizeWallet,
  initializePrivateKey,
  defaultPrivateKey,
  regenrateAccountEvent,
  fetchBalanceFx,
  $balance,
  topUpWalletBalanceFx,
  topUpEvent,
  fetchBalanceCurrenciesFx,
  $balanceCurrency,
} from "./models/model";
import { persist } from "effector-storage/local";
import {
  Faucet,
  HttpTransport,
  LocalECDSAKeySigner,
  PublicClient,
  WalletV1,
  convertEthToWei,
  generateRandomPrivateKey,
} from "@nilfoundation/niljs";
import { sendMethodFx } from "../contracts/model";

persist({
  store: $endpoint,
  key: "endpoint",
});

persist({
  store: $privateKey,
  key: "privateKey",
});

$privateKey.on(setPrivateKey, (_, privateKey) => privateKey);
$endpoint.on(setEndpoint, (_, endpoint) => endpoint);

createWalletFx.use(async ({ privateKey, endpoint }) => {
  const signer = new LocalECDSAKeySigner({ privateKey });
  const client = new PublicClient({ transport: new HttpTransport({ endpoint }) });
  const pubkey = await signer.getPublicKey();
  const wallet = new WalletV1({
    pubkey,
    salt: 100n,
    shardId: 1,
    client,
    signer,
  });

  const balance = await wallet.getBalance();
  if (balance === 0n) {
    const faucet = new Faucet(client);
    await faucet.withdrawToWithRetry(wallet.getAddressHex(), convertEthToWei(0.1));
  }

  const code = await client.getCode(wallet.getAddressHex());
  if (code.length === 0) {
    await wallet.selfDeploy(true);
  }

  return wallet;
});

topUpWalletBalanceFx.use(async (wallet) => {
  const faucet = new Faucet(wallet.client);
  await faucet.withdrawToWithRetry(wallet.getAddressHex(), convertEthToWei(0.1)); // 0.0001
  return await wallet.getBalance();
});

fetchBalanceFx.use(async (wallet) => {
  return await wallet.getBalance();
});

fetchBalanceCurrenciesFx.use(async (wallet) => {
  return await wallet.client.getCurrencies(wallet.getAddressHex(), "latest");
});

createWalletFx.failData.watch((error) => {
  console.error(error);
});

forward({
  from: $accountConnectorWithEndpoint,
  to: createWalletFx,
});

$wallet.reset($privateKey);
$wallet.on(createWalletFx.doneData, (_, wallet) => wallet);

sample({
  source: $accountConnectorWithEndpoint,
  clock: initilizeWallet,
  target: createWalletFx,
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
  clock: createWalletFx.doneData,
  target: fetchBalanceFx,
});

sample({
  clock: createWalletFx.doneData,
  target: fetchBalanceCurrenciesFx,
});

sample({
  clock: topUpEvent,
  target: topUpWalletBalanceFx,
  source: $wallet,
  filter: (wallet) => wallet !== null,
  fn: (wallet) => wallet as WalletV1,
});

$balance.on(fetchBalanceFx.doneData, (_, balance) => balance);
$balance.on(topUpWalletBalanceFx.doneData, (_, balance) => balance);
$balance.reset($wallet);

$balanceCurrency.on(fetchBalanceCurrenciesFx.doneData, (_, currencies) => currencies);
$balanceCurrency.reset($wallet);

initializePrivateKey();

initilizeWallet();

sample({
  clock: sendMethodFx.doneData,
  target: fetchBalanceFx,
  source: $wallet,
  filter: (wallet) => wallet !== null,
  fn: (wallet) => wallet as WalletV1,
});
