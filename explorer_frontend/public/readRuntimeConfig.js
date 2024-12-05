// This script is used to load the runtime configuration from runtime-config.toml
// and runtime-config.local.toml files.
// The runtime configuration is stored in the window.RUNTIME_CONFIG variable.

function fetchTomlFileSync(url) {
  var xhr = new XMLHttpRequest();
  xhr.open("GET", url, false);

  try {
    xhr.send(null);
    if (xhr.status === 200) {
      return xhr.responseText;
    }

    return null;
  } catch (error) {
    console.error("Error loading file:", url, "Error:", error);
    return null;
  }
}

function safeParseToml(t) {
  try {
    return tomlParser.parse(t);
  } catch (error) {
    return {};
  }
}

function loadConfig() {
  var config = {};
  var localConfig = {};

  var configToml = fetchTomlFileSync("/runtime-config.toml");
  var localConfigToml = fetchTomlFileSync("/runtime-config.local.toml");

  if (configToml) {
    config = safeParseToml(configToml);
  }

  if (localConfigToml) {
    localConfig = safeParseToml(localConfigToml);
  }

  var mergedConfig = { ...config, ...localConfig }; // override default values with local values
  window["RUNTIME_CONFIG"] = mergedConfig;
}

loadConfig();
