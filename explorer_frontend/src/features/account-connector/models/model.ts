import { combine, createDomain } from "effector";
import type { WalletV1, Hex } from "@nilfoundation/niljs";

export const explorerTransactionDomain = createDomain("account-connector");
const createStore = explorerTransactionDomain.createStore.bind(explorerTransactionDomain);
const createEvent = explorerTransactionDomain.createEvent.bind(explorerTransactionDomain);

export const defaultPrivateKey = "0x00000";
export const $privateKey = createStore<Hex>(defaultPrivateKey);
export const setPrivateKey = createEvent<Hex>();
export const initializePrivateKey = createEvent();
export const $wallet = createStore<WalletV1 | null>(null);
export const $balance = createStore<bigint | null>(null);
export const $balanceCurrency = createStore<Record<string, bigint> | null>(null);
export const $endpoint = createStore<string>("");

export const $accountConnectorWithEndpoint = combine($privateKey, $endpoint, (privateKey, endpoint) => ({
  privateKey,
  endpoint,
}));

export const setEndpoint = createEvent<string>();

export const fetchBalanceFx = explorerTransactionDomain.createEffect<WalletV1, bigint>();

export const fetchBalanceCurrenciesFx = explorerTransactionDomain.createEffect<WalletV1, Record<string, bigint>>();

export const createWalletFx = explorerTransactionDomain.createEffect<
  {
    privateKey: Hex;
    endpoint: string;
  },
  WalletV1
>();

export const topUpWalletBalanceFx = explorerTransactionDomain.createEffect<WalletV1, bigint>();

export const initilizeWallet = createEvent();

export const regenrateAccountEvent = createEvent();

export const topUpEvent = createEvent();
