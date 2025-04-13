import type { Hex, SmartAccountV1 } from "@nilfoundation/niljs";
import { combine, createDomain } from "effector";
import { Token } from "../tokens";
import { ActiveComponent } from "./ActiveComponent";

export const accountConnectorDomain = createDomain("account-connector");
const createStore = accountConnectorDomain.createStore.bind(accountConnectorDomain);
const createEvent = accountConnectorDomain.createEvent.bind(accountConnectorDomain);

export const defaultPrivateKey = "0x00000";
export const $privateKey = createStore<Hex>(defaultPrivateKey);
export const setPrivateKey = createEvent<Hex>();
export const initializePrivateKey = createEvent();
export const $smartAccount = createStore<SmartAccountV1 | null>(null);
export const $balance = createStore<bigint | null>(null);
export const $balanceToken = createStore<Record<string, bigint> | null>(null);
export const $rpcUrl = createStore<string>("");
export const $topUpError = createStore<string>("");

export const $latestActivity = createStore<{
  txHash: string;
  successful: boolean;
} | null>(null);

export const $accountConnectorWithRpcUrl = combine($privateKey, $rpcUrl, (privateKey, rpcUrl) => ({
  privateKey,
  rpcUrl,
}));

export const setRpcUrl = createEvent<string>();

export const fetchBalanceFx = accountConnectorDomain.createEffect<SmartAccountV1, bigint>();

export const fetchBalanceTokensFx = accountConnectorDomain.createEffect<
  SmartAccountV1,
  Record<string, bigint>
>();

export const createSmartAccountFx = accountConnectorDomain.createEffect<
  {
    privateKey: Hex;
    rpcUrl: string;
  },
  {
    smartAccount: SmartAccountV1;
    rpcUrl: string;
  }
>();

export const topUpSmartAccountBalanceFx = accountConnectorDomain.createEffect<
  SmartAccountV1,
  bigint
>();

export const initilizeSmartAccount = createEvent<string>();

export const regenrateAccountEvent = createEvent();

export const topUpEvent = createEvent();

export const $activeComponent = createStore<ActiveComponent | null>(ActiveComponent.RpcUrl);

export const setActiveComponent = createEvent<ActiveComponent>();

export const $topupInput = createStore<{
  token: string;
  amount: string;
}>({
  token: Token.NIL,
  amount: "",
});

export const setTopupInput = createEvent<{
  token: string;
  amount: string;
}>();

export const topupSmartAccountTokenFx = accountConnectorDomain.createEffect<
  {
    smartAccount: SmartAccountV1;
    topupInput: {
      token: string;
      amount: string;
    };
    faucets: Record<string, Hex>;
    rpcUrl: string;
  },
  void
>();

export const topupTokenEvent = accountConnectorDomain.createEvent();

export const $initializingSmartAccountState = accountConnectorDomain.createStore<string>("");

export const setInitializingSmartAccountState = accountConnectorDomain.createEvent<string>();

export const $initializingSmartAccountError = accountConnectorDomain.createStore<string>("");

export const resetTopUpError = createEvent();

export const addActivity = createEvent<{
  txHash: string;
  successful: boolean;
}>();

export const clearLatestActivity = createEvent();
