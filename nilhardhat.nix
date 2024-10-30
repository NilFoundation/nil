{ lib
, stdenv
, biome
, fetchFromGitHub
, fetchNpmDeps
, callPackage
, npmHooks
, nodejs
, nil
, enableTesting ? false
}:

stdenv.mkDerivation rec {
  name = "nil-hardhat-plugin";
  src = lib.sourceByRegex ./. [
    "package.json"
    "package-lock.json"
    "^hardhat-plugin(/.*)?$"
    "^niljs(/.*)?$"
    "^create-nil-hardhat-project(/.*)?$"
    "^smart-contracts(/.*)?$"
    "biome.json"
  ];

  soljson = builtins.fetchurl {
    url = "https://binaries.soliditylang.org/wasm/soljson-v0.8.26+commit.8a97fa7a.js";
    sha256 = "1mhww44ni55yfcyn4hjql2hwnvag40p78kac7jjw2g2jdwwyb1fv";
  };

  npmDeps = (callPackage ./npmdeps.nix { });

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [
    nodejs
    npmHooks.npmConfigHook
    nil
    biome
  ];

  dontConfigure = true;

  buildPhase = ''
    patchShebangs hardhat-plugin/node_modules
    (cd smart-contracts; npm run compile)
    (cd niljs; npm run build)
    cd hardhat-plugin
    npm run build
  '';

  doCheck = enableTesting;

  checkPhase = ''
    export BIOME_BINARY=${biome}/bin/biome

    echo "Linting hardhat-plugin"
    npm run lint

    cd ../create-nil-hardhat-project

    echo "Installing soljson"
    bash install_soljson.sh ${soljson}

    echo "Running tests"
    bash run_tests.sh
  '';

  installPhase = ''
    mkdir -p $out
    touch $out/dummy
  '';
}
