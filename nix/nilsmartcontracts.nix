{ lib
, stdenv
, callPackage
, pnpm_10
, nodejs
}:

stdenv.mkDerivation rec {
  name = "smart-contracts";
  pname = "smart-contracts";
  src = lib.sourceByRegex ./.. [
    "package.json"
    "pnpm-lock.yaml"
    "pnpm-workspace.yaml"
    ".npmrc"
    "^smart-contracts(/.*)?$"
  ];

  pnpmDeps = (callPackage ./npmdeps.nix { });

  nativeBuildInputs = [
    nodejs
    pnpm_10.configHook
  ];


  preUnpack = ''
    echo "Setting UV_USE_IO_URING=0 to work around the io_uring kernel bug"
    export UV_USE_IO_URING=0
  '';

  buildPhase = ''
    cd smart-contracts
    pnpm run build
  '';

  installPhase = ''
    mkdir -p $out
    touch $out/dummy
  '';
}
