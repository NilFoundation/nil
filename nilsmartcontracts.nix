{ lib
, stdenv
, callPackage
, npmHooks
, nodejs
}:

stdenv.mkDerivation rec {
  name = "smart-contracts";
  pname = "smart-contracts";
  src = lib.sourceByRegex ./. [
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

  buildPhase = ''
    cd smart-contracts
    npm run compile
  '';

  installPhase = ''
    mkdir -p $out
    touch $out/dummy
  '';
}
