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
  src = lib.sourceByRegex ./. [
    "package.json"
    "package-lock.json"
    "^hardhat-plugin(/.*)?$"
    "^niljs(/.*)?$"
    "^create-nil-hardhat-project(/.*)?$"
    "^smart-contracts(/.*)?$"
    "biome.json"
  ];

  soljson26 = builtins.fetchurl {
    url = "https://binaries.soliditylang.org/wasm/soljson-v0.8.26+commit.8a97fa7a.js";
    sha256 = "1mhww44ni55yfcyn4hjql2hwnvag40p78kac7jjw2g2jdwwyb1fv";
  };

  soljson21 = builtins.fetchurl {
    url = "https://binaries.soliditylang.org/wasm/soljson-v0.8.21+commit.d9974bed.js";
    sha256 = "05ss7jgcfb4zlgmnyln95g7i0ghxxzfn56a336g0610xni9a7gj5";
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
    bash install_soljson.sh ${soljson26} ${soljson21}

    echo "Running hardhat-examples tests"
    bash run_tests.sh

    # Do this hack so that solc-select thinks we have solc-0.8.21 installed
    export HOME="$TMPDIR"
    mkdir -p ~/.gsolc-select/artifacts/solc-0.8.21
    ln -f -s ${solc}/bin/solc ~/.gsolc-select/artifacts/solc-0.8.21/solc-0.8.21

    cd ../hardhat-plugin
    echo "Running hardhat-plugin tests"
    bash test/run_tests.sh nild cometa
  '';

  installPhase = ''
    mkdir -p $out
    touch $out/dummy
  '';
}
