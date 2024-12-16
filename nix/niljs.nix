{ lib
, stdenv
, biome
, callPackage
, npmHooks
, nodejs
, nil
, solc
, enableTesting ? false
}:

stdenv.mkDerivation rec {
  name = "nil.js";
  pname = "niljs";
  src = lib.sourceByRegex ./.. [ "package.json" "package-lock.json" "^niljs(/.*)?$" "^smart-contracts(/.*)?$" "biome.json" ];

  npmDeps = (callPackage ./npmdeps.nix { });

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [
    nodejs
    npmHooks.npmConfigHook
    biome
    solc
  ] ++ (if enableTesting then [ nil ] else [ ]);

  dontConfigure = true;

  preUnpack = ''
    echo "Setting UV_USE_IO_URING=0 to work around the io_uring kernel bug"
    export UV_USE_IO_URING=0
  '';

  buildPhase = ''
    (cd smart-contracts; npm run build)
    cd niljs
    npm run build
  '';

  doCheck = enableTesting;

  checkPhase = ''
    patchShebangs node_modules
    nohup nild run --http-port 8529 --collator-tick-ms=100 > nild.log 2>&1 & echo $! > nild_pid &
    nohup faucet run > faucet.log 2>&1 & echo $! > faucet_pid

    export BIOME_BINARY=${biome}/bin/biome

    npm run lint
    npm run test:unit
    npm run test:integration --cache=false
    npm run test:examples
    npm run lint:types
    npm run lint:jsdoc

    kill `cat nild_pid` && rm nild_pid
    kill `cat faucet_pid` && rm faucet_pid

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
