{ pkgs, lib, stdenv, biome, callPackage, pnpm_10, nil, enableTesting ? false }:

let
  sigtool = callPackage ./sigtool.nix { };
  nodejs_static = pkgs.pkgsStatic.nodejs_22;
  pnpm_static = (pnpm_10.override { nodejs = nodejs_static; });
in
stdenv.mkDerivation rec {
  name = "clijs";
  pname = "clijs";
  src = lib.sourceByRegex ./.. [
    "package.json"
    "pnpm-workspace.yaml"
    "pnpm-lock.yaml"
    ".npmrc"
    "^clijs(/.*)?$"
    "^niljs(/.*)?$"
    "^smart-contracts(/.*)?$"
    "biome.json"
  ];

  pnpmDeps = (callPackage ./npmdeps.nix { });

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [ nodejs_static pnpm_static.configHook biome ]
    ++ lib.optionals stdenv.buildPlatform.isDarwin [ sigtool ]
    ++ (if enableTesting then [ nil ] else [ ]);

  preUnpack = ''
    echo "Setting UV_USE_IO_URING=0 to work around the io_uring kernel bug"
    export UV_USE_IO_URING=0
  '';

  postUnpack = ''
    mkdir source/nil
    cp -R ${nil}/contracts source/nil
  '';

  buildPhase = ''
    PATH="${nodejs_static}/bin/:$PATH"

    patchShebangs docs/node_modules
    patchShebangs niljs/node_modules
    (cd smart-contracts; pnpm run build)
    (cd niljs; pnpm run build)

    cd clijs
    pnpm run bundle
  '';

  doCheck = enableTesting;

  checkPhase = ''
    export BIOME_BINARY=${biome}/bin/biome

    pnpm run lint

    ./dist/clijs | grep -q "The CLI tool for interacting with the =nil; cluster" || {
      echo "Error: Output does not contain the expected substring!" >&2
      exit 1
    }
    echo "smoke check passed"

    nohup nild run --http-port 8529 --collator-tick-ms=100 > nild.log 2>&1 & echo $! > nild_pid &

    pnpm run test:ci

    kill `cat nild_pid` && rm nild_pid

    echo "tests finished successfully"
  '';

  installPhase = ''
    mkdir -p $out
    mv ./dist/clijs $out/${pname}
  '';
}
