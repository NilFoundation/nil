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
  name = "wallet extension";
  pname = "walletExtension";
  src = lib.sourceByRegex ./.. [
    "package.json"
    "package-lock.json"
    "^niljs(/.*)?$"
    "^smart-contracts(/.*)?$"
    "biome.json"
    "^wallet-extension(/.*)?$"
  ];

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

    echo "Setting npm_config_ignore_scripts=true to ignore npm lifecycle scripts"
    export npm_config_ignore_scripts=true
  '';

  buildPhase = ''
    patchShebangs wallet-extension/node_modules

    (cd smart-contracts; npm run build)
    (cd niljs; npm run build)

    cd wallet-extension
    npm run build
  '';

  doCheck = enableTesting;

  checkPhase = ''
    patchShebangs node_modules
    nohup nild run --http-port 8529 --collator-tick-ms=100 > nild.log 2>&1 & echo $! > nild_pid &

    export BIOME_BINARY=${biome}/bin/biome

    echo "Checking wallet extension"

    npm run lint
    npm run test:integration --cache=false

    kill `cat nild_pid` && rm nild_pid

    echo "tests finished successfully"
  '';

  installPhase = ''
    mkdir -p $out
    mkdir -p $out/dist
    cp -r package.json $out
    cp -r src $out
    cp -r dist/* $out/dist
  '';
}
