{ lib, stdenv, npmHooks, nodejs, nil, openssl, fetchNpmDeps, callPackage, autoconf, automake, libtool, solc, solc-select, enableTesting ? false }:


stdenv.mkDerivation rec {
  name = "nil.docs";
  pname = "nildocs";
  src = lib.sourceByRegex ./. [
    "package.json"
    "package-lock.json"
    "^docs(/.*)?$"
    "^niljs(/.*)?$"
    "^smart-contracts(/.*)?$"
  ];

  npmDeps = (callPackage ./npmdeps.nix { });

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
    solc-select
  ];

  dontConfigure = true;

  preBuild = ''
    export HOME="$TMPDIR"
    mkdir -p ~/.gsolc-select/artifacts/solc-0.8.21
    ln -f -s ${solc}/bin/solc ~/.gsolc-select/artifacts/solc-0.8.21/solc-0.8.21
  '';

  buildPhase = ''
    runHook preBuild
    patchShebangs docs/node_modules
    patchShebangs niljs/node_modules
    (cd smart-contracts; npm run compile)
    (cd niljs; npm run build)
    export NILJS_SRC=${./niljs}
    export OPENRPC_JSON=${nil}/share/doc/nil/openrpc.json
    export CMD_NIL=${./nil/cmd/nil/internal}
    export COMETA_CONFIG=${./docs/tests/cometa.yaml}
    cd docs
    npm run build
    solc --version
  '';


  doCheck = enableTesting;

  checkPhase = ''
    echo "Runnig tests..."
    bash run_tests.sh
    echo "Tests passed"
  '';

  shellHook = ''
    export NILJS_SRC=${./niljs}
    export OPENRPC_JSON=${nil}/share/doc/nil/openrpc.json
    export CMD_NIL=${./nil/cmd/nil/internal}
    export COMETA_CONFIG=${./docs/tests/cometa.yaml}
    mkdir -p ~/.solc-select/artifacts/solc-0.8.21
    ln -f -s ${solc}/bin/solc ~/.solc-select/artifacts/solc-0.8.21/solc-0.8.21
  '';

  installPhase = ''
    mv build $out
  '';
}
