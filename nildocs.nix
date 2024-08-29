{ lib, stdenv, npmHooks, nodejs, nil, openssl, fetchNpmDeps, autoconf, automake, libtool, solc, enableTesting ? false }:

stdenv.mkDerivation rec {
  name = "nil.docs";
  pname = "nildocs";
  src = lib.sourceByRegex ./. [
    "package.json"
    "package-lock.json"
    "^docs(/.*)?$"
    "^niljs(/.*)?$"
  ];

  npmDeps = fetchNpmDeps {
    inherit src;
    hash = "sha256-b4fjrq6rhg+6PR0ZVjtMzSNIEsW8npjZ8yeThIcIxZk=";
  };

  NODE_PATH = "$npmDeps";

  buildInputs = [
    openssl
  ];

  nativeBuildInputs = [
    nodejs
    nil
    npmHooks.npmConfigHook
    autoconf
    automake
    libtool
    solc
  ];

  dontConfigure = true;

  buildPhase = ''
    patchShebangs docs/node_modules
    (cd niljs; npm run build)

    export NILJS_SRC=${./niljs}
    export OPENRPC_JSON=${nil}/share/doc/nil/openrpc.json
    export NODE_OPTIONS=--openssl-legacy-provider

    cd docs
    npm run build
  '';

  doCheck = enableTesting;

  checkPhase = ''
    echo "Runnig tests..."
    bash run_tests.sh
    echo "Tests passed"
  '';

  shellHook = ''
    export NILJS_SRC=${./niljs}
  '';

  installPhase = ''
    mv build $out
  '';
}
