{ lib, stdenv, callPackage, pnpm_10, nodejs }:

stdenv.mkDerivation rec {
  name = "aibackend";
  pname = "docsaibackend";
  src = lib.sourceByRegex ./.. [
    "package.json"
    "pnpm-lock.yaml"
    "pnpm-workspace.yaml"
    ".npmrc"
    "^docs_ai_backend(/.*)?$"
    "^smart-contracts(/.*)?$"
  ];

  pnpmDeps = (callPackage ./npmdeps.nix { });

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [ nodejs nodejs.python pnpm_10 pnpm_10.configHook ];

  preUnpack = ''
    echo "Setting UV_USE_IO_URING=0 to work around the io_uring kernel bug"
    export UV_USE_IO_URING=0
  '';

  buildPhase = ''
    patchShebangs docs_ai_backend/node_modules

    (cd docs_ai_backend; pnpm run build)
  '';

  checkPhase = "";

  installPhase = ''
    mkdir -p $out
    mv smart-contracts $out/smart-contracts
    mv node_modules $out/node_modules
    mv docs_ai_backend/ $out/docs_ai_backend
  '';
}
