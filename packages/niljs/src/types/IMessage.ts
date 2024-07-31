import type { ValueOf } from "@chainsafe/ssz";
import type { SszMessageSchema } from "../encoding/ssz.js";

/**
 * The interface for the message object. This object is used to represent a message in the client code.
 * It may differ from the actual message object used inside the network.
 */
interface IMessage extends ValueOf<typeof SszMessageSchema> {}

export type { IMessage };
