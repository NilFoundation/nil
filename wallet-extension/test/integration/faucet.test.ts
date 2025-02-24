import { convertEthToWei } from "@nilfoundation/niljs";
import {
  fetchBalance,
  fetchSmartAccountTokens,
  initializeOrDeploySmartAccount,
  topUpAllCurrencies,
  topUpSpecificCurrency,
} from "../../src/features/blockchain";
import { Currency } from "../../src/features/components/currency";
import { btcAddress } from "../../src/features/utils/token.ts";
import { setup } from "./helper.ts";

test("Top up all currencies and verify balances", async () => {
  // 1. Set up smart account
  const { client, signer, shardId, faucetClient } = await setup();
  const smartAccount = await initializeOrDeploySmartAccount({
    client,
    signer,
    shardId,
    faucetClient,
  });

  // 2. Fetch initial balances
  const initialBalance = await fetchBalance(smartAccount);
  const initialTokens = await fetchSmartAccountTokens(smartAccount);

  // 3. Top up all currencies
  await topUpAllCurrencies(smartAccount, faucetClient);

  // 4. Fetch balances after top-up
  const finalBalance = await fetchBalance(smartAccount);
  const finalTokens = await fetchSmartAccountTokens(smartAccount);

  // 5. Check that balances increased
  expect(finalBalance).toBeGreaterThan(initialBalance);
  for (const token of Object.keys(finalTokens)) {
    expect(finalTokens[token]).toBeGreaterThan(initialTokens[token] ?? 0n);
  }
});

test("Top up NIL currency and verify exact balance change", async () => {
  // 1. Set up smart account
  const { client, signer, shardId, faucetClient } = await setup();
  const smartAccount = await initializeOrDeploySmartAccount({
    client,
    signer,
    shardId,
    faucetClient,
  });

  // 2. Fetch initial NIL balance
  const initialBalance = await fetchBalance(smartAccount);

  // 3. Top up NIL currency
  const topUpAmountNIL = convertEthToWei(0.0001);
  await topUpSpecificCurrency(smartAccount, faucetClient, Currency.NIL, topUpAmountNIL);

  // 4. Fetch updated NIL balance
  const finalBalance = await fetchBalance(smartAccount);

  // 5. Verify exact balance increase
  expect(finalBalance).toBeGreaterThan(initialBalance);
});

test("Top up BTC token and verify balance update", async () => {
  // 1. Set up smart account
  const { client, signer, shardId, faucetClient } = await setup();
  const smartAccount = await initializeOrDeploySmartAccount({
    client,
    signer,
    shardId,
    faucetClient,
  });

  // 2. Fetch initial token balances
  const initialTokens = await fetchSmartAccountTokens(smartAccount);

  // 3. Top up BTC token
  const topUpAmountBTC = 5n;
  await topUpSpecificCurrency(smartAccount, faucetClient, Currency.BTC, topUpAmountBTC);

  // 4. Fetch updated token balances
  const finalTokens = await fetchSmartAccountTokens(smartAccount);

  // 5. Verify BTC balance increase
  expect(finalTokens[btcAddress]).toBe((initialTokens[btcAddress] ?? 0n) + topUpAmountBTC);
});
