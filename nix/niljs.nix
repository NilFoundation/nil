{ lib
, stdenv
, biome
, callPackage
, npmHooks
, nodejs
, pnpm
, nil
, solc
, enableTesting ? false
}:

stdenv.mkDerivation rec {
  name = "nil.js";
  pname = "niljs";
  src = lib.sourceByRegex ./.. [ "package.json" ".npmrc" "pnpm-workspace.yaml" "pnpm-lock.yaml" "^niljs(/.*)?$" "^smart-contracts(/.*)?$" "biome.json" ];

  pnpmDeps = (callPackage ./npmdeps.nix { });


  nativeBuildInputs = [
    nodejs
    biome
    pnpm
    pnpm.configHook
    solc
  ] ++ (if enableTesting then [ nil ] else [ ]);

  preUnpack = ''
    echo "Setting UV_USE_IO_URING=0 to work around the io_uring kernel bug"
    export UV_USE_IO_URING=0
  '';

  buildPhase = ''
    (cd smart-contracts; pnpm run build)
    cd niljs
    pnpm run build
  '';

  doCheck = enableTesting;

  checkPhase = ''
    patchShebangs node_modules
    nohup nild run --http-port 8529 --collator-tick-ms=100 > nild.log 2>&1 & echo $! > nild_pid &

    export BIOME_BINARY=${biome}/bin/biome

    pnpm run lint
    pnpm run test:unit
    pnpm run test:integration --cache=false
    pnpm run test:examples
    pnpm run lint:types
    pnpm run lint:jsdoc

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
