{ lib, stdenv, fetchFromGitHub, fetchNpmDeps, npmHooks, nodejs, nil }:

stdenv.mkDerivation rec {
  name = "nil.js";
  pname = "niljs";
  src = lib.sourceByRegex ./. ["package.json" "package-lock.json" "^niljs(/.*)?$"];

  npmDeps = fetchNpmDeps {
    inherit src;
    hash = "sha256-+KfATAYbBW5SMrrul08mZ1A04WuPIjOA7IurDDP17d0=";
  };

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [
    nodejs
    npmHooks.npmConfigHook
    nil
  ];

  dontConfigure = true;

  buildPhase = ''
    cd niljs
    nohup nild run > nild.log 2>&1 & echo $! > nild_pid
    npm run test:unit
    npm run test:integration --cache=false
    npm run test:examples
    npm run build
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
