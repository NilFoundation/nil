import { presignMessagePrefix } from "viem";

const PREFIX = Uint8Array.from(presignMessagePrefix.split("").map((c) => c.charCodeAt(0)));

export const prepareSignMessage = (message: Uint8Array) => {
  const m = new Uint8Array(PREFIX.length + message.length + 1);
  m.set(PREFIX, 0);
  m.set([message.length], PREFIX.length);
  m.set(message, PREFIX.length + 1);
  return m;
};
