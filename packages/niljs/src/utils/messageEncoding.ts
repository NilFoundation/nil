import { ExternalMessageEnvelope } from "../message.js";
import type { ISigner } from "../signers/index.js";
import type { ExternalMessage } from "../types/ExternalMessage.js";

/**
 * Encodes the external message.
 *
 * @async
 * @param {Omit<ExternalMessage, "authData">} params The external message without its auth data.
 * @param {ISigner} signer The message signer.
 * @returns {Promise<{ raw: Uint8Array; hash: Uint8Array }>} The message bytecode and the message hash.
 */
export const externalMessageEncode = async (
  params: Omit<ExternalMessage, "authData">,
  signer: ISigner,
): Promise<{ raw: Uint8Array; hash: Uint8Array }> => {
  const message = new ExternalMessageEnvelope({
    ...params,
    authData: new Uint8Array(0),
  });
  const res = await message.encodeWithSignature(signer);
  return res;
};
