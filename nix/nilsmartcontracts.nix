{ lib
, stdenv
, callPackage
, npmHooks
, nodejs
}:

stdenv.mkDerivation rec {
  name = "smart-contracts";
  pname = "smart-contracts";
  src = lib.sourceByRegex ./.. [
    "package.json"
    "package-lock.json"
    "^smart-contracts(/.*)?$"
  ];

  npmDeps = (callPackage ./npmdeps.nix { });

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [
    nodejs
    npmHooks.npmConfigHook
  ];

  dontConfigure = true;

  preUnpack = ''
    echo "Setting UV_USE_IO_URING=0 to work around the io_uring kernel bug"
    export UV_USE_IO_URING=0
  '';

  buildPhase = ''
    cd smart-contracts
    npm run build
  '';

  installPhase = ''
    mkdir -p $out
    touch $out/dummy
  '';
}
