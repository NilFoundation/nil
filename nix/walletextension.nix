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
  name = "wallet extension";
  pname = "walletExtension";
  src = lib.sourceByRegex ./.. [
    "package.json"
    "pnpm-lock.yaml"
    "pnpm-workspace.yaml"
    ".npmrc"
    "^niljs(/.*)?$"
    "^smart-contracts(/.*)?$"
    "biome.json"
    "^wallet-extension(/.*)?$"
  ];

  pnpmDeps = (callPackage ./npmdeps.nix { });

  nativeBuildInputs = [
    nodejs
    pnpm_10.configHook
    pnpm_10
    biome
  ] ++ (if enableTesting then [ nil ] else [ ]);


  buildPhase = ''
    (cd smart-contracts; pnpm run build)
    (cd niljs; pnpm run build)

    cd wallet-extension
    pnpm run build
  '';

  doCheck = enableTesting;

  checkPhase = ''
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
