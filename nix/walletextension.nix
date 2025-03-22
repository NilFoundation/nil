{ lib, stdenv, biome, callPackage, npmHooks, nodejs, nil, enableTesting ? false
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

  nativeBuildInputs = [ nodejs npmHooks.npmConfigHook biome ]
    ++ (if enableTesting then [ nil ] else [ ]);

  dontConfigure = true;

  buildPhase = ''
    patchShebangs wallet-extension/node_modules

    (cd smart-contracts; pnpm run build)
    (cd niljs; pnpm run build)

    cd wallet-extension
    pnpm run build
  '';

  doCheck = enableTesting;

  checkPhase = ''
    patchShebangs node_modules
    nohup nild run --http-port 8529 --collator-tick-ms=100 > nild.log 2>&1 & echo $! > nild_pid &

    export BIOME_BINARY=${biome}/bin/biome

    echo "Checking wallet extension"

    pnpm run lint
    pnpm run test:integration --cache=false

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
