{ lib
, stdenv
, biome
, python3
, callPackage
, npmHooks
, nodejs
, enableTesting ? false
, cypress
}:

stdenv.mkDerivation rec {
  name = "explorer";
  pname = "nilexplorer";
  src = lib.sourceByRegex ./.. [
    "package.json"
    "package-lock.json"
    "^niljs(/.*)?$"
    "^smart-contracts(/.*)?$"
    "biome.json"
    "^explorer_frontend(/.*)?$"
    "^explorer_backend(/.*)?$"
  ];

  npmDeps = (callPackage ./npmdeps.nix { });

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [
    nodejs
    npmHooks.npmConfigHook
    biome
    python3
  ];

  dontConfigure = true;

  preUnpack = ''
    echo "Setting UV_USE_IO_URING=0 to work around the io_uring kernel bug"
    export UV_USE_IO_URING=0

    export CYPRESS_INSTALL_BINARY=0
    export CYPRESS_RUN_BINARY=${cypress}/bin/Cypress
  '';

  buildPhase = ''
    patchShebangs explorer_frontend/node_modules

    (cd smart-contracts; npm run build)
    (cd niljs; npm run build)

    (cd explorer_frontend; npm run build)
    (cd explorer_backend; npm run build)
  '';

  doCheck = enableTesting;

  checkPhase = ''
    export BIOME_BINARY=${biome}/bin/biome

    echo "Checking explorer frontend"
    (cd explorer_frontend; npm run lint;)

    echo "Checking explorer backend"
    (cd explorer_backend; npm run lint;)

    echo "Checking if explorer backend starts up without errors"
    cd explorer_backend
    npm run start & NPM_PID=$!
    sleep 7

    if kill -0 $NPM_PID 2>/dev/null; then
      echo "Explorer backend is running successfully"
    else
      echo "Explorer backend startup failed"
      exit 1
    fi

    kill $NPM_PID
    cd -

    echo "tests finished successfully"
  '';

  installPhase = ''
    mkdir -p $out
    mv explorer_frontend/ $out/frontend
    mv explorer_backend/ $out/backend
  '';
}
