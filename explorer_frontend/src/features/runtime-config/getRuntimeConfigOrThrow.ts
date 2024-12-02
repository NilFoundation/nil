const getRuntimeConfig = () => window.RUNTIME_CONFIG;

const runtimeConfigFiles = ["/runtime-config.toml", "/runtime-config.local.toml"];

export const getRuntimeConfigOrThrow = () => {
  const config = getRuntimeConfig();

  if (!config) {
    throw new Error(
      `Runtime config not found. Expected to find it in one of the following files: ${runtimeConfigFiles.join(", ")}`,
    );
  }

  return config;
};
