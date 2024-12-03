const keys = [
  "DOCUMENTATION_URL",
  "GITHUB_URL",
  "API_URL",
  "HOTJAR_ID",
  "HOTJAR_VERSION",
  "SANDBOX_DOCS_URL",
  "SANDBOX_NILJS_URL",
  "SANDBOX_MULTICURRENCY_URL",
  "EXPLORER_USAGE_DOCS_URL",
] as const;

type RuntimConfigKeys = (typeof keys)[number];

declare global {
  interface Window {
    RUNTIME_CONFIG: Record<RuntimConfigKeys, string>;
  }
}

export {};
