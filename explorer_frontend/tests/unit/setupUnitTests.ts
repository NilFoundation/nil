import "@testing-library/jest-dom";
import { vi } from "vitest";

Object.defineProperty(window, "RUNTIME_CONFIG", {
  value: {
    DOCUMENTATION_URL: "https://docs.nil.foundation/nil/intro",
    GITHUB_URL: "https://github.com/NilFoundation/nil-hardhat-example",
    API_URL: "https://explore.nil.foundation/api",
    COMETA_SERVICE_API_URL: "https://api.devnet.nil.foundation/api",
    RPC_TELEGRAM_BOT: "https://t.me/NilDevnetTokenBot",
    RPC_API_URL: "https://api.devnet.nil.foundation/api",
    PLAYGROUND_DOCS_URL: "https://docs.nil.foundation",
    PLAYGROUND_NILJS_URL: "https://github.com/NilFoundation/nil.js",
    PLAYGROUND_MULTI_TOKEN_URL:
      "https://docs.nil.foundation/nil/getting-started/essentials/tokens",
    PLAYGROUND_SUPPORT_URL: "https://t.me/+PT-6HyWK_LBmMmIx",
    PLAYGROUND_FEEDBACK_URL: "https://form.typeform.com/to/pDEAcSqd",
    API_REQUESTS_ENABLE_BATCHING: true,
    RECENT_PROJECTS_STORAGE_LIMIT: 5,
  },
  writable: true,
});

Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: vi.fn().mockImplementation((query) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
});
