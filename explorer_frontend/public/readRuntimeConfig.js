function fetchTomlFileSync(url) {
  var expectedContentType = "application/toml";
  var xhr = new XMLHttpRequest();
  xhr.open("GET", url, false);

  try {
    xhr.send(null);
    if (xhr.status === 200) {
      var contentType = xhr.getResponseHeader("Content-Type");
      if (contentType !== expectedContentType) {
        return null;
      }

      return xhr.responseText;
    }

    return null;
  } catch (error) {
    console.error("Error loading file:", url, "Error:", error);
    return null;
  }
}

function safeParseToml(toml) {
  try {
    return tomlParser.parse(toml);
  } catch (error) {
    return {};
  }
}

function loadConfig() {
  var config = {};
  var localConfig = {};

  var configToml = fetchTomlFileSync("./runtime-config.toml");
  var localConfigToml = fetchTomlFileSync("./runtime-config.local.toml");

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
