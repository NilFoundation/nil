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

  forgeStd = pkgs.fetchzip {
    url = "https://github.com/foundry-rs/forge-std/archive/refs/tags/v1.9.6.zip";
    sha256 = "sha256-4y1Hf0Te2oJxwKBOgVBEHZeKYt7hs+wTgdIO+rItj0E=";
  };

  solmate = pkgs.fetchFromGitHub {
    owner = "transmissions11";
    repo = "solmate";
    rev = "c93f7716c9909175d45f6ef80a34a650e2d24e56";
    sha256 = "sha256-zv8Jzap34N5lFVZV/zoT/fk73pSLP/eY427Go3QQM/Y="; # Replace with actual hash
  };

  dsTest = pkgs.fetchFromGitHub {
    owner = "dapphub";
    repo = "ds-test";
    rev = "e282159d5170298eb2455a6c05280ab5a73a4ef0";
    sha256 = "sha256-wXtNq4ZUohndNGs9VttOI9m9VW5QlVKOPtR8+mv2fBM=";
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

    echo "Copying Foundry libraries from Nix store"
    mkdir -p lib/forge-std lib/solmate lib/ds-test
    cp -r ${forgeStd}/* lib/forge-std
    cp -r ${solmate}/* lib/solmate
    cp -r ${dsTest}/* lib/ds-test

    echo "Installing Node.js dependencies..."
    npm install  # Ensure node_modules exists before compiling

    echo "Start Hardhat compiling:"
    #npx hardhat clean && npx hardhat compile

    echo "Start Forge compiling:"
    ./forge_command_proxy.sh test
  '';

  installPhase = ''
    mkdir -p $out
    cp -r * $out/
    cp .env $out/
  '';
}

