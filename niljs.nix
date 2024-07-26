{ stdenv, bun, fetchFromGitHub, fetchNpmDeps, npmHooks, nodejs, nil }:

stdenv.mkDerivation rec {
  name = "nil.js";
  src = fetchFromGitHub {
    owner = "NilFoundation";
    repo = "nil.js";
    rev = "v0.11.0";
    sha256 =
      "sha256-ED0Yt1dYuj1cTmezgq1lnCklTnnypKdIu0ZoXTskES8="; # replace with the actual sha256
  };

  buildInputs = [ bun ];

  npmDeps = fetchNpmDeps {
    inherit src;
    hash = "sha256-4hdWPUOhs2BIeNKhChj6NbTUsN6s2jcmJFsSApKCb9s=";
  };

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [
    nodejs
    npmHooks.npmConfigHook
    nil
  ];

  dontConfigure = true;

  buildPhase = ''
    nohup nil run > nil.log 2>&1 & echo $! > nil_pid
    CI=true bunx vitest run -c test/vitest.integration.config.ts --cache=false --testTimeout=40000
    kill `cat nil_pid` && rm nil_pid
    echo "tests finished successfully"
  '';

  installPhase = ''
    mkdir -p $out
    touch $out/dummy
    mkdir -p $out/src
    cp -r $src/* $out/src
  '';
}
