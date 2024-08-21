{ lib, stdenv, npmHooks, nodejs, nil, openssl, fetchNpmDeps, autoconf, automake, libtool
, enableTesting ? false }:

stdenv.mkDerivation rec {
  name = "nil.docs";
  pname = "nildocs";
  src = lib.sourceByRegex ./. ["package.json" "package-lock.json" "^docs(/.*)?$"];
  buildInputs = [ nodejs npmHooks.npmConfigHook openssl ] ;

  npmDeps = fetchNpmDeps {
    inherit src;
    hash = "sha256-zhXhpGiwKfe6/4/p7F8ccHqsmj4Cq68xpShIILWfydw=";
  };

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [
    nodejs
    nil
    npmHooks.npmConfigHook
    autoconf
    automake
    libtool
  ];

  dontConfigure = true;

  postPatch = ''
    export HOME=$NIX_BUILD_TOP/fake_home
    patchShebangs node_modules/
  '';

  buildPhase = ''
    patchShebangs hardhat-plugin/node_modules

    export NILJS_SRC=${./niljs}
    export OPENRPC_JSON=${nil}/share/doc/nil/openrpc.json
    export NODE_OPTIONS=--openssl-legacy-provider
    npm run build --legacy-peer-deps --workspaces

    cd docs
  '';

  doCheck = enableTesting;

  checkPhase = ''
    echo "Run tests here"
  '';

  shellHook = ''
    export NILJS_SRC=${./niljs}
  '';

  installPhase = ''
    mv build $out
  '';
}
