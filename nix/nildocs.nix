{ lib
, stdenv
, pnpm_10
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
    "pnpm-lock.yaml"
    "pnpm-workspace.yaml"
    ".npmrc"
    "^docs(/.*)?$"
    "^niljs(/.*)?$"
    "^smart-contracts(/.*)?$"
    "biome.json"
  ];

  pnpmDeps = (callPackage ./npmdeps.nix { });

  buildInputs = [
    openssl
  ];

  nativeBuildInputs = [
    nodejs
    pnpm_10
    pnpm_10.configHook
    autoconf
    automake
    libtool
    solc
    solc-select
    biome
  ] ++ (if enableTesting then [ nil ] else [ ]);

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
    (cd smart-contracts; pnpm run build)
    (cd niljs; pnpm run build)
    export NILJS_SRC=${../niljs}
    export OPENRPC_JSON=${nil}/share/doc/nil/openrpc.json
    export CMD_NIL=${../nil/cmd/nil/internal}
    export NIL_CLI=${nil}/bin/nil
    export COMETA_CONFIG=${../docs/tests/cometa.yaml}
    export NODE_JS=${nodejs}/bin/node
    export NIL=${nil}
    cd docs

    # needed to work-around the openssl incompatibility
    # not sure why it happens, but it does the job
    export NODE_OPTIONS=--openssl-legacy-provider
    pnpm run build
  '';


  doCheck = enableTesting;

  checkPhase = ''
    export BIOME_BINARY=${biome}/bin/biome

    pnpm run lint

    echo "Runnig tests..."
    bash run_tests.sh
    echo "Tests passed"
  '';

  shellHook = ''
    export NILJS_SRC=${../niljs}
    export NIL_CLI=${nil}/bin/nil
    export OPENRPC_JSON=${nil}/share/doc/nil/openrpc.json
    export CMD_NIL=${../nil/cmd/nil/internal}
    export COMETA_CONFIG=${../docs/tests/cometa.yaml}
    export NODE_JS=${nodejs}/bin/node
    mkdir -p ~/.solc-select/artifacts/solc-0.8.28
    ln -f -s ${solc}/bin/solc ~/.solc-select/artifacts/solc-0.8.28/solc-0.8.28
    export NODE_OPTIONS=--openssl-legacy-provider
  '';

  installPhase = ''
    mv build $out
  '';
}
