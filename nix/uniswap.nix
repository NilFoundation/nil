{ lib
, stdenv
, biome
, callPackage
, npmHooks
, nodejs
, nil
, enableTesting ? false
}:

stdenv.mkDerivation rec {
  name = "uniswap";
  pname = "uniswap";
  src = lib.sourceByRegex ./.. [
    "package.json"
    "package-lock.json"
    "^niljs(/.*)?$"
    "^smart-contracts(/.*)?$"
    "^hardhat-plugin(/.*)?$"
    "^create-nil-hardhat-project(/.*)?$"
    "biome.json"
    "^uniswap(/.*)?$"
  ];

  soljson26 = builtins.fetchurl {
    url = "https://binaries.soliditylang.org/wasm/soljson-v0.8.26+commit.8a97fa7a.js";
    sha256 = "1mhww44ni55yfcyn4hjql2hwnvag40p78kac7jjw2g2jdwwyb1fv";
  };

  npmDeps = (callPackage ./npmdeps.nix { });

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [
    nodejs
    npmHooks.npmConfigHook
    biome
  ] ++ (if enableTesting then [ nil ] else [ ]);

  dontConfigure = true;

  preUnpack = ''
    echo "Setting UV_USE_IO_URING=0 to work around the io_uring kernel bug"
    export UV_USE_IO_URING=0
  '';

  buildPhase = ''
    patchShebangs node_modules

    (cd smart-contracts; npm run compile)
    (cd niljs; npm run build)
    (cd hardhat-plugin; npm run build)
  '';

  doCheck = enableTesting;

  checkPhase = ''
    echo "Installing soljson"
    (cd create-nil-hardhat-project; bash install_soljson.sh ${soljson26})

    export BIOME_BINARY=${biome}/bin/biome
    export NIL_RPC_ENDPOINT="http://127.0.0.1:8529"

    nild run --http-port 8529 --collator-tick-ms=100 >nild.log 2>&1 &
    faucet run &
    sleep 2

    nil config set rpc_endpoint $NIL_RPC_ENDPOINT
    nil config set faucet_endpoint http://127.0.0.1:8527

    export PRIVATE_KEY=`nil keygen new -q`
    export WALLET_ADDR=`nil wallet new -q`
    export FAUCET_ADDR="000100000000000000000000000000000FA00CE7"

    echo "Checking uniswap"
    (cd uniswap; npm run lint)
    (cd uniswap; npm run compile)

    echo "tests finished successfully"
  '';

  installPhase = ''
    mkdir -p $out
    touch $out/dummy
  '';
}
