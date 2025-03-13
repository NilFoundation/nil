{ lib
, stdenv
, callPackage
, npmHooks
, nodejs
, nil
, pkgs
, enableTesting ? false
}:

stdenv.mkDerivation rec {
  name = "rollup-bridge-contracts";
  pname = "rollup-bridge-contracts";
  src = lib.sourceByRegex ./.. [
    "package.json"
    "package-lock.json"
    "^niljs(/.*)?$"
    "^rollup-bridge-contracts(/.*)?$"
    "^create-nil-hardhat-project(/.*)?$"
  ];

  npmDeps = (callPackage ./npmdeps.nix { });

  nativeBuildInputs = [
    nodejs
    npmHooks.npmConfigHook
    pkgs.foundry
    pkgs.solc
  ];

  soljson26 = builtins.fetchurl {
    url = "https://binaries.soliditylang.org/wasm/soljson-v0.8.26+commit.8a97fa7a.js";
    sha256 = "1mhww44ni55yfcyn4hjql2hwnvag40p78kac7jjw2g2jdwwyb1fv";
  };

  buildPhase = ''
    echo "Installing soljson"
    (cd create-nil-hardhat-project; bash install_soljson.sh ${soljson26})

    export FOUNDRY_SOLC=$(command -v solc)
    export FOUNDRY_ROOT=$(realpath ../)

    echo "Versions:"
    forge --version
    cast --version
    anvil --version

    cd rollup-bridge-contracts
    pwd
    cp .env.example .env

    echo "Installing Node.js dependencies..."
    npm install  # Ensure node_modules exists before compiling

    echo "Start Hardhat compiling:"
    #npx hardhat clean && npx hardhat compile

    echo "Start Forge compiling:"
    forge compile --root $FOUNDRY_ROOT

    echo "Start Forge testing:"
    forge test $(realpath . ) -vvvvv
  '';

  installPhase = ''
    mkdir -p $out
    cp -r * $out/
    cp .env $out/
  '';
}

