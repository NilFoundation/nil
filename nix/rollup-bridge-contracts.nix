{ lib
, stdenv
, biome
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
    "biome.json"
    "^create-nil-hardhat-project(/.*)?$"
  ];

  npmDeps = (callPackage ./npmdeps.nix { });

  nativeBuildInputs = [
    nodejs
    npmHooks.npmConfigHook
    pkgs.foundry
    pkgs.solc
    pkgs.bash
  ];

  soljson26 = builtins.fetchurl {
    url = "https://binaries.soliditylang.org/wasm/soljson-v0.8.28+commit.7893614a.js";
    sha256 = "0ip1kafi7l5zkn69zj5c41al7s947wqr8llf08q33565dq55ivvj";
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
    export BIOME_BINARY=${biome}/bin/biome

    echo "Versions:"
    solc --version
    forge --version
    cast --version

    cd rollup-bridge-contracts
    cp .env.example .env

    echo "Start Hardhat compiling:"
    npx hardhat clean && npx hardhat compile
  '';

  doCheck = enableTesting;
  checkPhase = ''
    source .env

    export FOUNDRY_ROOT=$(realpath ../)
    export FOUNDRY_SOLC=$(command -v solc)
    echo "FOUNDRY_SOLC=$FOUNDRY_SOLC"

    echo "Versions:"
    solc --version
    forge --version

    echo "Copying Foundry libraries from Nix store"
    mkdir -p lib/forge-std lib/solmate lib/ds-test
    ln -s ${forgeStd}/* lib/forge-std
    ln -s ${solmate}/* lib/solmate
    ln -s ${dsTest}/* lib/ds-test

    eval forge test --remappings hardhat/=/build/source/node_modules/hardhat/ --remappings forge-std/=rollup-bridge-contracts/lib/forge-std/src/ --remappings ds-test/=rollup-bridge-contracts/lib/ds-test/src/ --remappings solmate/=rollup-bridge-contracts/lib/solmate/src/ --remappings @openzeppelin/=/build/source/node_modules/@openzeppelin/ --remappings @solady/=/build/source/node_modules/solady/src/
  '';

  installPhase = ''
    mkdir -p $out
    cp ../package-lock.json $out/
    cp -r * $out/
    cp .env $out/
    rm -rf $out/node_modules
    rm -rf $out/cache
  '';
}

