import type { ITransport } from "../../transport/types/ITransport.js";
import type { ISigner } from "../../signers/types/ISigner.js";

/**
 * The client configuration that is shared between public and private clients.
 */
type IClientBaseConfig = {
	/**
	 * The ID of the shard with which the client interacts.
	 * @example 1
	 */
	shardId?: number;
	/**
	 * The transport is used to send requests to the network.
	 * @example
	 * import { MetaMaskTransport } from '@nilfoundation/niljs';
	 *
	 * const transport = new MetaMaskTransport();
	 *
	 * const client = new PublicClient({ transport, shardId: 1 });
	 */
	transport: ITransport;
};

/**
 * The type representing the config for the public client.
 */
type IPublicClientConfig = IClientBaseConfig;

/**
 * The type representing the config for the basic wallet.
 */
type IWalletClientConfig = IClientBaseConfig & {
	/**
	 * An instance of Signer is used to sign messages.
	 * If a Signer is absent from the config, messages should be signed explicitly before being passed to the client.
	 * @example
	 * import { Signer } from '@nilfoundation/niljs';
	 *
	 * const signer = new Signer();
	 *
	 * const client = new WalletClient({
	 *  endpoint: 'http://127.0.0.1:8529',
	 *  signer: signer
	 * })
	 */
	signer: ISigner;
};

/**
 * The type representing the config for the faucet client.
 */
type FaucetClientConfig = IClientBaseConfig;

/**
 * The type representing the config for the Cometa service client.
 */
type CometaServiceConfig = IClientBaseConfig

export type {
	IClientBaseConfig,
	IPublicClientConfig,
	IWalletClientConfig,
	FaucetClientConfig,
	CometaServiceConfig,
};
