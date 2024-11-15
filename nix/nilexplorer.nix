{ lib
, stdenv
, biome
, python3
, callPackage
, npmHooks
, nodejs
, enableTesting ? false
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

  buildPhase = ''
    patchShebangs explorer_frontend/node_modules

    (cd smart-contracts; npm run compile)
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

    echo "tests finished successfully"
  '';

  installPhase = ''
    mkdir -p $out
    touch $out/dummy
  '';
}
