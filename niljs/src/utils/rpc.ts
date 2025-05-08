import { version } from "../version.js";

const isValidHttpHeaders = (headers: unknown) => {
  if (headers === null || typeof headers !== "object" || Array.isArray(headers)) {
    throw new Error("Invalid headers provided.");
  }

  const isValidObj = Object.entries(headers).every(
    ([key, value]) => typeof key === "string" && typeof value === "string",
  );

  if (!isValidObj) {
    throw new Error("Invalid http headers provided.");
  }
};

const requestHeadersWithDefaults = (headers: Record<string, string> = {}) => {
  isValidHttpHeaders(headers);

  const defaultHeaders = {
    "Client-Version": `niljs/${version}`,
    "Content-Type": "application/json",
    Accept: "application/json",
  };

  return { ...defaultHeaders, ...headers };
};

export { requestHeadersWithDefaults };
