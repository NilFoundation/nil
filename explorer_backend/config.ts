import "dotenv/config";

const defaults = {
  RPC_URL: "http://127.0.0.1:8529",
  DB_URL: "http://127.0.0.1:9000",
  DB_USER: "root",
  DB_PASSWORD: "",
  DB_NAME: "fiddle",
  DB_PATHNAME: "/",
  PORT: 3000,
  METER_EXPORTER_URL: null,
  JWT_SECRET_KEY: "sesfsasafc1241ret_afsafsaffagtsskey_safasaffassf",
  JWT_EXPIRES_IN: "1h",
  JWT_ALGORITHM: "HS256",
  TRACE_SAMPLE_RATIO: 0.1,
  OTLP_PROTOCOL: "http",
  TRACE_EXPORTER_URL: null,
  INTERVAL_CACHE_CHECKER: 1000,
  CACHE_DEADLINE: 5000,
  EXPLORER_CODE_SNIPPETS_DB_PATH: "./database.db",
};

export const config = {
  DB_URL: process.env.DB_URL || defaults.DB_URL,
  RPC_URL: process.env.RPC_URL || defaults.RPC_URL,
  DB_USER: process.env.DB_USER || defaults.DB_USER,
  DB_PASSWORD: process.env.DB_PASSWORD || defaults.DB_PASSWORD,
  DB_NAME: process.env.DB_NAME || defaults.DB_NAME,
  DB_PATHNAME: process.env.DB_PATHNAME || defaults.DB_PATHNAME,
  PORT: process.env.PORT ? +process.env.PORT : defaults.PORT,
  METER_EXPORTER_URL: process.env.HTTP_METER_EXPORTER_URL ?? defaults.METER_EXPORTER_URL,
  TRACE_EXPORTER_URL: process.env.HTTP_TRACE_EXPORTER_URL ?? defaults.TRACE_EXPORTER_URL,
  JWT_SECRET_KEY: process.env.JWT_SECRET_KEY || defaults.JWT_SECRET_KEY,
  JWT_EXPIRES_IN: process.env.JWT_EXPIRES_IN || defaults.JWT_EXPIRES_IN,
  JWT_ALGORITHM: process.env.JWT_ALGORITHM || defaults.JWT_ALGORITHM,
  TRACE_SAMPLE_RATIO: process.env.TRACE_SAMPLE_RATIO
    ? +process.env.TRACE_SAMPLE_RATIO
    : defaults.TRACE_SAMPLE_RATIO,
  OTLP_PROTOCOL: process.env.OTLP_PROTOCOL || defaults.OTLP_PROTOCOL,
  INTERVAL_CACHE_CHECKER: process.env.INTERVAL_CACHE_CHECKER
    ? +process.env.INTERVAL_CACHE_CHECKER
    : defaults.INTERVAL_CACHE_CHECKER,
  CACHE_DEADLINE: process.env.CACHE_DEADLINE
    ? +process.env.CACHE_DEADLINE
    : defaults.CACHE_DEADLINE,
  EXPLORER_CODE_SNIPPETS_DB_PATH:
    process.env.EXPLORER_CODE_SNIPPETS_DB_PATH || defaults.EXPLORER_CODE_SNIPPETS_DB_PATH,
} as const;
