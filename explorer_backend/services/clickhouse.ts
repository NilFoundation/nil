import { createClient } from "@clickhouse/client";
import { config } from "../config";

if (!config.DB_URL) throw new Error("DB_URL is not set");

export const client = createClient({
  url: config.DB_URL,
  username: config.DB_USER,
  pathname: config.DB_PATHNAME,
  password: config.DB_PASSWORD,
  database: config.DB_NAME,
});
