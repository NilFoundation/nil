import { combine, createDomain } from "effector";
import type { WalletV1, Hex } from "@nilfoundation/niljs";
import { ActiveComponent } from "../ActiveComponent";
import { Currency } from "../../currencies";

export const accountConnectorDomain = createDomain("account-connector");
const createStore = accountConnectorDomain.createStore.bind(
	accountConnectorDomain,
);
const createEvent = accountConnectorDomain.createEvent.bind(
	accountConnectorDomain,
);

export const defaultPrivateKey = "0x00000";
export const $privateKey = createStore<Hex>(defaultPrivateKey);
export const setPrivateKey = createEvent<Hex>();
export const initializePrivateKey = createEvent();
export const $wallet = createStore<WalletV1 | null>(null);
export const $balance = createStore<bigint | null>(null);
export const $balanceCurrency = createStore<Record<string, bigint> | null>(
	null,
);
export const $endpoint = createStore<string>("");

export const $accountConnectorWithEndpoint = combine(
	$privateKey,
	$endpoint,
	(privateKey, endpoint) => ({
		privateKey,
		endpoint,
	}),
);

export const setEndpoint = createEvent<string>();

export const fetchBalanceFx = accountConnectorDomain.createEffect<
	WalletV1,
	bigint
>();

export const fetchBalanceCurrenciesFx = accountConnectorDomain.createEffect<
	WalletV1,
	Record<string, bigint>
>();

export const createWalletFx = accountConnectorDomain.createEffect<
	{
		privateKey: Hex;
		endpoint: string;
		faucetEndpoint: string;
	},
	WalletV1
>();

export const topUpWalletBalanceFx = accountConnectorDomain.createEffect<
	WalletV1,
	bigint
>();

export const initilizeWallet = createEvent();

export const regenrateAccountEvent = createEvent();

export const topUpEvent = createEvent();

export const $activeComponent = createStore<ActiveComponent | null>(
	ActiveComponent.Main,
);

export const setActiveComponent = createEvent<ActiveComponent>();

export const $topupInput = createStore<{
	currency: string;
	amount: string;
}>({
	currency: Currency.ETH,
	amount: "",
});

export const setTopupInput = createEvent<{
	currency: string;
	amount: string;
}>();

export const topupWalletCurrencyFx = accountConnectorDomain.createEffect<
	{
		wallet: WalletV1;
		topupInput: {
			currency: string;
			amount: string;
		};
		faucets: Record<string, Hex>;
		endpoint: string;
		faucetEndpoint: string;
	},
	void
>();

export const topupCurrencyEvent = accountConnectorDomain.createEvent();

export const $initializingWalletState =
	accountConnectorDomain.createStore<string>("");

export const setInitializingWalletState =
	accountConnectorDomain.createEvent<string>();

export const $initializingWalletError =
	accountConnectorDomain.createStore<string>("");
