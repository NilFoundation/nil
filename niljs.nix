{ lib
, stdenv
, fetchFromGitHub
, fetchNpmDeps
, callPackage
, npmHooks
, nodejs
, nil
, enableTesting ? false
}:

stdenv.mkDerivation rec {
  name = "nil.js";
  pname = "niljs";
  src = lib.sourceByRegex ./. [ "package.json" "package-lock.json" "^niljs(/.*)?$" ];

  npmDeps = (callPackage ./npmdeps.nix { });

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [
    nodejs
    npmHooks.npmConfigHook
    nil
  ];

  dontConfigure = true;

  buildPhase = ''
    cd niljs
    npm run build
  '';

  doCheck = enableTesting;

  checkPhase = ''
    npm run test:unit

    nohup nild run > nild.log 2>&1 & echo $! > nild_pid
    npm run test:integration --cache=false
    npm run test:examples
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
