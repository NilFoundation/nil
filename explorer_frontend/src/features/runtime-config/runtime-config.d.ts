// this files enables autocomplete for runtime config values

const keys = [
  "DOCUMENTATION_URL",
  "GITHUB_URL",
  "API_URL",
  "PLAYGROUND_DOCS_URL",
  "PLAYGROUND_NILJS_URL",
  "PLAYGROUND_MULTI_TOKEN_URL",
  "PLAYGROUND_SUPPORT_URL",
  "PLAYGROUND_FEEDBACK_URL",
  "COMETA_SERVICE_API_URL",
  "RPC_TELEGRAM_BOT",
  "RPC_API_URL",
  "API_REQUESTS_ENABLE_BATCHING",
  "RECENT_PROJECTS_STORAGE_LIMIT",
  "PLAYGROUND_VERSION",
] as const;

type RuntimConfigKeys = (typeof keys)[number];

declare global {
  interface Window {
    RUNTIME_CONFIG: Record<RuntimConfigKeys, string>;
  }
}

export {};
