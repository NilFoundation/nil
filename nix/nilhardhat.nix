{ lib
, stdenv
, biome
, callPackage
, pnpm_10
, nodejs
, nil
, enableTesting ? false
, solc
, solc-select
}:

stdenv.mkDerivation rec {
  name = "nil-hardhat-plugin";
  pname = "nil-hardhat-plugin";
  src = lib.sourceByRegex ./.. [
    "package.json"
    "pnpm-lock.yaml"
    "pnpm-workspace.yaml"
    ".npmrc"
    "^niljs(/.*)?$"
    "^smart-contracts(/.*)?$"
    "^create-nil-hardhat-project(/.*)?$"
    "biome.json"
    "^hardhat-plugin(/.*)?$"
  ];

  soljson26 = builtins.fetchurl {
    url = "https://binaries.soliditylang.org/wasm/soljson-v0.8.26+commit.8a97fa7a.js";
    sha256 = "1mhww44ni55yfcyn4hjql2hwnvag40p78kac7jjw2g2jdwwyb1fv";
  };

  pnpmDeps = (callPackage ./npmdeps.nix { });

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [
    nodejs
    pnpm_10
    pnpm_10.configHook
    biome
    solc
  ] ++ (if enableTesting then [ nil ] else [ ]);

  preUnpack = ''
    echo "Setting UV_USE_IO_URING=0 to work around the io_uring kernel bug"
    export UV_USE_IO_URING=0
  '';

  buildPhase = ''
    patchShebangs node_modules || true

    (cd smart-contracts; pnpm run build)
    (cd niljs; pnpm run build)

    (cd hardhat-plugin && pnpm run build)
  '';

  doCheck = enableTesting;

  checkPhase = ''
    export BIOME_BINARY=${biome}/bin/biome

    echo "Linting hardhat-plugin"
    (cd hardhat-plugin; pnpm run lint)
  '';

  installPhase = ''
    mkdir -p $out
    touch $out/dummy
  '';
}
