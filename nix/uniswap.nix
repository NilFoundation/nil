{ lib
, stdenv
, biome
, callPackage
, pnpm_10
, nodejs
, nil
, enableTesting ? false
}:

stdenv.mkDerivation rec {
  name = "uniswap";
  pname = "uniswap";
  src = lib.sourceByRegex ./.. [
    "package.json"
    "pnpm-lock.yaml"
    "pnpm-workspace.yaml"
    ".npmrc"
    "^niljs(/.*)?$"
    "^smart-contracts(/.*)?$"
    "^create-nil-hardhat-project(/.*)?$"
    "biome.json"
    "^uniswap(/.*)?$"
  ];

  soljson26 = builtins.fetchurl {
    url = "https://binaries.soliditylang.org/wasm/soljson-v0.8.26+commit.8a97fa7a.js";
    sha256 = "1mhww44ni55yfcyn4hjql2hwnvag40p78kac7jjw2g2jdwwyb1fv";
  };

  pnpmDeps = (callPackage ./npmdeps.nix { });


  nativeBuildInputs = [
    nodejs
    pnpm_10.configHook
    pnpm_10
    biome
  ] ++ (if enableTesting then [ nil ] else [ ]);

  preUnpack = ''
    echo "Setting UV_USE_IO_URING=0 to work around the io_uring kernel bug"
    export UV_USE_IO_URING=0
  '';

  buildPhase = ''
    patchShebangs node_modules

    (cd smart-contracts; pnpm run build)
    (cd niljs; pnpm run build)
  '';

  doCheck = enableTesting;

  checkPhase = ''
    echo "Installing soljson"
    (cd create-nil-hardhat-project; bash install_soljson.sh ${soljson26})

    export BIOME_BINARY=${biome}/bin/biome
    export NIL_RPC_ENDPOINT="http://127.0.0.1:8529"

    nild run --http-port 8529 --collator-tick-ms=100 >nild.log 2>&1 &
    sleep 2

    nil config set rpc_endpoint $NIL_RPC_ENDPOINT

    export PRIVATE_KEY=`nil keygen new -q`
    export SMART_ACCOUNT_ADDR=`nil smart-account new -q`

    echo "Checking uniswap"
    (cd uniswap; pnpm run lint)
    (cd uniswap; pnpm run compile)

    echo "tests finished successfully"
  '';

  installPhase = ''
    mkdir -p $out
    touch $out/dummy
  '';
}
