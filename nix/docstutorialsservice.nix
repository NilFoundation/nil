{ lib
, stdenv
, callPackage
, npmHooks
, nodejs
}:

stdenv.mkDerivation rec {
  name = "tutorialsservice";
  pname = "docstutorialsservice";
  src = lib.sourceByRegex ./.. [
    "package.json"
    "package-lock.json"
    "^docs_interactive_tutorials_service(/.*)?$"
  ];

  npmDeps = (callPackage ./npmdeps.nix { });

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [
    nodejs
    npmHooks.npmConfigHook
    nodejs.python
  ];

  dontConfigure = true;

  preUnpack = ''
    echo "Setting UV_USE_IO_URING=0 to work around the io_uring kernel bug"
    export UV_USE_IO_URING=0
  '';

  buildPhase = ''
    patchShebangs docs_interactive_tutorials_service/node_modules

    (cd docs_interactive_tutorials_service; npm run build)
  '';

  checkPhase = ''
  '';

  installPhase = ''
    mkdir -p $out
    mv docs_interactive_tutorials_service/ $out/docs_interactive_tutorials_service
  '';
}
