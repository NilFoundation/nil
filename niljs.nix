{ stdenv, bun, fetchFromGitHub, fetchNpmDeps, npmHooks, nodejs, nil }:

stdenv.mkDerivation rec {
  name = "nil.js";
  pname = "niljs";
  src = ./niljs;

  buildInputs = [ bun ];

  npmDeps = fetchNpmDeps {
    inherit src;
    hash = "sha256-sswv+Vpj4lgK/Sb8KNj5N080A5F0WXQvP9IU2Q2EfII=";
  };

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [
    nodejs
    npmHooks.npmConfigHook
    nil
  ];

  dontConfigure = true;

  buildPhase = ''
    nohup nild run > nild.log 2>&1 & echo $! > nild_pid
    bun run test:unit
    bun run test:integration --cache=false
    bun run build
    kill `cat nild_pid` && rm nild_pid
    echo "tests finished successfully"
  '';

  installPhase = ''
    mkdir -p $out
    mkdir -p $out/dist
    cp -r package.json $out
    cp -r package-lock.json $out
    cp -r src $out
    cp -r dist/* $out/dist
  '';
}
