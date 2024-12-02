{ lib
, stdenv
, biome
, callPackage
, npmHooks
, nodejs
, nil
, enableTesting ? false
, solc
, solc-select
}:

stdenv.mkDerivation rec {
  name = "nil-hardhat-plugin";
  src = lib.sourceByRegex ./.. [
    "package.json"
    "package-lock.json"
    "^hardhat-plugin(/.*)?$"
    "^niljs(/.*)?$"
    "^create-nil-hardhat-project(/.*)?$"
    "^smart-contracts(/.*)?$"
    "biome.json"
  ];

  # All versions are listed here: https://binaries.soliditylang.org/wasm/list.json
  soljson26 = builtins.fetchurl {
    url = "https://binaries.soliditylang.org/wasm/soljson-v0.8.26+commit.8a97fa7a.js";
    sha256 = "1mhww44ni55yfcyn4hjql2hwnvag40p78kac7jjw2g2jdwwyb1fv";
  };

  soljson28 = builtins.fetchurl {
    url = "https://binaries.soliditylang.org/wasm/soljson-v0.8.28+commit.7893614a.js";
    sha256 = "0ip1kafi7l5zkn69zj5c41al7s947wqr8llf08q33565dq55ivvj";
  };

  npmDeps = (callPackage ./npmdeps.nix { });

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [
    nodejs
    npmHooks.npmConfigHook
    biome
  ] ++ (if enableTesting then [ nil ] else [ ]);

  dontConfigure = true;

  buildPhase = ''
    export UV_USE_IO_URING=0
    patchShebangs hardhat-plugin/node_modules
    (cd smart-contracts; npm run compile)
    (cd niljs; npm run build)
    cd hardhat-plugin
    npm run build
  '';

  doCheck = enableTesting;

  checkPhase = ''
    export UV_USE_IO_URING=0
    export BIOME_BINARY=${biome}/bin/biome

    echo "Linting hardhat-plugin"
    npm run lint

    cd ../create-nil-hardhat-project

    echo "Installing soljson"
    bash install_soljson.sh ${soljson26} ${soljson28}

    echo "Running hardhat-examples tests"
    bash run_tests.sh

    # Do this hack so that solc-select thinks we have solc-0.8.28 installed
    export HOME="$TMPDIR"
    mkdir -p ~/.gsolc-select/artifacts/solc-0.8.28
    ln -f -s ${solc}/bin/solc ~/.gsolc-select/artifacts/solc-0.8.28/solc-0.8.28

    cd ../hardhat-plugin
    echo "Running hardhat-plugin tests"
    bash test/run_tests.sh nild cometa
  '';

  installPhase = ''
    mkdir -p $out
    touch $out/dummy
  '';
}
