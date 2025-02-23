import { handleConnectRequest, handleConnectionResponse } from "./connection";
import { ACTIONS, PORTS } from "./constants.ts";
import { handleProcessRequest, handleSendSignResponse } from "./transaction";
import { focusOrCreateWelcomeTab } from "./util.ts";

// Map to store requestId -> port mappings
const requestPortMap = new Map<string, chrome.runtime.Port>();

chrome.runtime.onInstalled.addListener(async ({ reason }) => {
  if (reason === "install") {
    await focusOrCreateWelcomeTab();
  }
});

chrome.commands.onCommand.addListener(async (command) => {
  if (command === "open-popup") {
    await chrome.action.openPopup();
  }
});

// Function to handle extension-handler messages
const handleExtensionHandlerMessage = async (port: chrome.runtime.Port) => {
  port.onMessage.addListener(async (message) => {
    const { action, request } = message;
    const requestId = request?.requestId;

    if (requestId) {
      requestPortMap.set(requestId, port);
    }

    switch (action) {
      case ACTIONS.CONNECT:
        await handleConnectRequest(requestId, request, port);
        break;

      case ACTIONS.PROCESS_REQUEST:
        await handleProcessRequest(requestId, request, port);
        break;

      default:
        console.warn("Unknown action received:", action);
    }
  });

  // Handle port disconnection and attempt reconnection
  port.onDisconnect.addListener(() => {
    reconnectPort(port.name);
  });
};

// Function to reconnect the port
const reconnectPort = (portName: string) => {
  // Avoid reconnecting if the port was closed by user action
  if (!chrome.runtime?.id) {
    return;
  }

  setTimeout(() => {
    const newPort = chrome.runtime.connect({ name: portName });
    if (portName === PORTS.EXTENSION_HANDLER) {
      handleExtensionHandlerMessage(newPort);
    } else if (portName === PORTS.CONNECTION_REQUEST) {
      handleConnectionRequestMessage(newPort);
    } else if (portName === PORTS.SIGNSEND_REQUEST) {
      handleSendSignRequestMessage(newPort);
    }
  }, 1000); // Reconnect after 1 second
};

// Handle sednSign request messages from popup
const handleSendSignRequestMessage = (port: chrome.runtime.Port) => {
  port.onMessage.addListener((message) => {
    const { requestId } = message;
    const targetPort = requestPortMap.get(requestId);
    if (targetPort) {
      handleSendSignResponse(message, targetPort);
      requestPortMap.delete(requestId);
    } else {
      console.error(`No port found for requestId: ${requestId}`);
    }
  });

  port.onDisconnect.addListener(() => {
    if (chrome.runtime?.lastError) {
      console.error(`Unexpected disconnection: ${chrome.runtime.lastError}`);
      reconnectPort(port.name);
      return;
    }
  });
};

// Handle connection request messages from popup
const handleConnectionRequestMessage = (port: chrome.runtime.Port) => {
  port.onMessage.addListener((message) => {
    const { requestId } = message;
    const targetPort = requestPortMap.get(requestId);
    if (targetPort) {
      handleConnectionResponse(message, targetPort);
      requestPortMap.delete(requestId);
    } else {
      console.error(`No port found for requestId: ${requestId}`);
    }
  });

  port.onDisconnect.addListener(() => {
    if (chrome.runtime?.lastError) {
      console.error(`Unexpected disconnection: ${chrome.runtime.lastError}`);
      reconnectPort(port.name);
      return;
    }
  });
};

// Listen for port connections
chrome.runtime.onConnect.addListener((port) => {
  if (port.name === PORTS.EXTENSION_HANDLER) {
    handleExtensionHandlerMessage(port);
  }

  if (port.name === PORTS.CONNECTION_REQUEST) {
    handleConnectionRequestMessage(port);
  }

  if (port.name === PORTS.SIGNSEND_REQUEST) {
    handleSendSignRequestMessage(port);
  }
});
