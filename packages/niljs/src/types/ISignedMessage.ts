import type { ISignature } from "../signers/index.js";
import type { IMessage } from "./IMessage.js";

/**
 * The signed message interface.
 *
 * @typedef {ISignedMessage}
 */
type ISignedMessage = IMessage & ISignature;

export type { ISignedMessage };
