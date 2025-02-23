import { addWindowMessageListener } from "../background/messagePassing/messageUtils.ts";
import { ExtensionMethodHandler } from "./methodHandler/ExtensionMethodHandler.ts";
import { type WindowNilRequest, isValidWindowNilRequest } from "./types.ts";

const extensionMethodHandler = new ExtensionMethodHandler();

addWindowMessageListener<WindowNilRequest>(isValidWindowNilRequest, async (request, source) => {
  // Include the website origin when sending the request
  const origin = window.location.origin;

  // Pass the origin with the request
  await extensionMethodHandler.handleRequest({
    ...request,
    origin: origin,
  });

  return;
});
