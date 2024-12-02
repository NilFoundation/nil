const getRuntimeConfig = () => window.RUNTIME_CONFIG;

export const getRuntimeConfigOrThrow = () => {
  const config = getRuntimeConfig();

  if (!config) {
    throw new Error("Runtime config is not set");
  }

  return config;
};
