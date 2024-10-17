import { combine, forward, sample } from "effector";
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
	$activeComponent,
	setActiveComponent,
	$topupInput,
	setTopupInput,
	topupWalletCurrencyFx,
	topupCurrencyEvent,
} from "./models/model";
import { persist as persistLocalStorage } from "effector-storage/local";
import { persist as persistSessionStorage } from "effector-storage/session";
import {
	Faucet,
	FaucetClient,
	type Hex,
	HttpTransport,
	LocalECDSAKeySigner,
	PublicClient,
	WalletV1,
	convertEthToWei,
	generateRandomPrivateKey,
} from "@nilfoundation/niljs";
import { sendMethodFx } from "../contracts/model";
import { $faucets, $faucetsEndpoint } from "../currencies/model";
import { sandboxRoute, sandboxWithHashRoute } from "../routing";
import { loadedPage } from "../code/model";

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

createWalletFx.use(async ({ privateKey, endpoint }) => {
	const signer = new LocalECDSAKeySigner({ privateKey });
	const client = new PublicClient({
		transport: new HttpTransport({ endpoint }),
	});
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
		await faucet.withdrawToWithRetry(
			wallet.getAddressHex(),
			convertEthToWei(0.1),
		);
	}

	const code = await client.getCode(wallet.getAddressHex());
	if (code.length === 0) {
		await wallet.selfDeploy(true);
	}

	return wallet;
});

topUpWalletBalanceFx.use(async (wallet) => {
	const faucet = new Faucet(wallet.client);
	await faucet.withdrawToWithRetry(
		wallet.getAddressHex(),
		convertEthToWei(0.1),
	); // 0.0001
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

$balanceCurrency.on(
	fetchBalanceCurrenciesFx.doneData,
	(_, currencies) => currencies,
);
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

$activeComponent.on(setActiveComponent, (_, payload) => payload);

persistSessionStorage({
	store: $activeComponent,
	key: "activeComponentWallet",
});

$topupInput.on(setTopupInput, (_, payload) => payload);

topupWalletCurrencyFx.use(
	async ({ wallet, topupInput, faucets, endpoint, faucetEndpoint }) => {
		const { currency, amount } = topupInput;
		const faucetClient = new FaucetClient({
			transport: new HttpTransport({ endpoint: faucetEndpoint }),
		});

		const publicClient = new PublicClient({
			transport: new HttpTransport({
				endpoint,
			}),
		});

		const currencyFaucetAddress = faucets[currency];

		await faucetClient.topUpAndWaitUntilCompletion(
			{
				walletAddress: wallet.getAddressHex(),
				faucetAddress: currencyFaucetAddress,
				amount: Number(amount),
			},
			publicClient,
		);
	},
);

sample({
	clock: topupCurrencyEvent,
	source: combine(
		$wallet,
		$topupInput,
		$faucets,
		$endpoint,
		$faucetsEndpoint,
		(wallet, topupInput, faucets, endpoint, faucetEndpoint) =>
			({
				wallet,
				topupInput,
				faucets,
				endpoint,
				faucetEndpoint,
			}) as {
				wallet: WalletV1;
				topupInput: { currency: string; amount: string };
				faucets: Record<string, Hex>;
				endpoint: string;
				faucetEndpoint: string;
			},
	),
	target: topupWalletCurrencyFx,
});

sample({
	clock: topupWalletCurrencyFx.doneData,
	target: fetchBalanceCurrenciesFx,
	source: $wallet,
	fn: (wallet) => wallet as WalletV1,
	filter: (wallet) => wallet !== null,
});

sample({
	clock: topupWalletCurrencyFx.doneData,
	target: fetchBalanceFx,
	source: $wallet,
	fn: (wallet) => wallet as WalletV1,
	filter: (wallet) => wallet !== null,
});

sample({
	clock: loadedPage,
	source: combine(
		sandboxRoute.$query,
		sandboxWithHashRoute.$query,
		(query1, query2) => {
			const user = query1.user ?? query2.user;
			const token = query1.token ?? query2.token;
			return { user, token };
		},
	),
	fn: (q) => {
		const user = q.user;
		const token = q.token;
		return `https://api.devnet.nil.foundation/api/${user}/${token}`;
	},
	filter: (q) => !!q.user && !!q.token,
	target: setEndpoint,
});

sample({
	clock: sendMethodFx.doneData,
  source: $wallet,
  fn: (wallet) => wallet as WalletV1,
  filter: (wallet) => wallet !== null,
  target: [fetchBalanceFx, fetchBalanceCurrenciesFx],
});
