{ lib
, stdenv
, npmHooks
, nodejs
, nil
, openssl
, callPackage
, autoconf
, automake
, libtool
, biome
, solc
, solc-select
, enableTesting ? false
}:

stdenv.mkDerivation rec {
  name = "nil.docs";
  pname = "nildocs";
  src = lib.sourceByRegex ./.. [
    "package.json"
    "package-lock.json"
    "^docs(/.*)?$"
    "^niljs(/.*)?$"
    "^smart-contracts(/.*)?$"
    "biome.json"
  ];

  npmDeps = (callPackage ./npmdeps.nix { });

  NODE_PATH = "$npmDeps";

  buildInputs = [
    openssl
  ];

  nativeBuildInputs = [
    nodejs
    npmHooks.npmConfigHook
    autoconf
    automake
    libtool
    solc
    solc-select
    biome
  ] ++ (if enableTesting then [ nil ] else [ ]);

  dontConfigure = true;

  preUnpack = ''
    echo "Setting UV_USE_IO_URING=0 to work around the io_uring kernel bug"
    export UV_USE_IO_URING=0
  '';

  preBuild = ''
    export HOME="$TMPDIR"
    mkdir -p ~/.gsolc-select/artifacts/solc-0.8.28
    ln -f -s ${solc}/bin/solc ~/.gsolc-select/artifacts/solc-0.8.28/solc-0.8.28
  '';

  buildPhase = ''
    runHook preBuild
    patchShebangs docs/node_modules
    patchShebangs niljs/node_modules
    (cd smart-contracts; npm run build)
    (cd niljs; npm run build)
    export NILJS_SRC=${../niljs}
    export OPENRPC_JSON=${nil}/share/doc/nil/openrpc.json
    export CMD_NIL=${../nil/cmd/nil/internal}
    export COMETA_CONFIG=${../docs/tests/cometa.yaml}
    export NODE_JS=${nodejs}/bin/node
    cd docs
    npm run build
  '';


  doCheck = enableTesting;

  checkPhase = ''
    export BIOME_BINARY=${biome}/bin/biome

    npm run lint

    echo "Runnig tests..."
    bash run_tests.sh
    echo "Tests passed"
  '';

  shellHook = ''
    export NILJS_SRC=${../niljs}
    export OPENRPC_JSON=${nil}/share/doc/nil/openrpc.json
    export CMD_NIL=${../nil/cmd/nil/internal}
    export COMETA_CONFIG=${../docs/tests/cometa.yaml}
    export NODE_JS=${nodejs}/bin/node
    mkdir -p ~/.solc-select/artifacts/solc-0.8.28
    ln -f -s ${solc}/bin/solc ~/.solc-select/artifacts/solc-0.8.28/solc-0.8.28
  '';

  installPhase = ''
    mv build $out
  '';
}
