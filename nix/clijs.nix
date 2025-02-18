{ pkgs
, lib
, stdenv
, biome
, callPackage
, npmHooks
, nil
, enableTesting ? false
}:

let
  sigtool = callPackage ./sigtool.nix { };
in
stdenv.mkDerivation rec {
  name = "clijs";
  pname = "clijs";
  src = lib.sourceByRegex ./.. [
    "package.json"
    "package-lock.json"
    "^clijs(/.*)?$"
    "^niljs(/.*)?$"
    "^smart-contracts(/.*)?$"
    "biome.json"
  ];

  npmDeps = (callPackage ./npmdeps.nix { });

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [
    pkgs.pkgsStatic.nodejs_22
    npmHooks.npmConfigHook
    biome
  ]
  ++ lib.optionals stdenv.buildPlatform.isDarwin [ sigtool ]
  ++ (if enableTesting then [ nil ] else [ ]);

  dontConfigure = true;

  preUnpack = ''
    echo "Setting UV_USE_IO_URING=0 to work around the io_uring kernel bug"
    export UV_USE_IO_URING=0

    echo "Setting npm_config_ignore_scripts=true to ignore npm lifecycle scripts"
    export npm_config_ignore_scripts=true
  '';

  postUnpack = ''
    mkdir source/nil
    cp -R ${nil}/contracts source/nil
  '';

  buildPhase = ''
    PATH="${pkgs.pkgsStatic.nodejs_22}/bin/:$PATH"

    patchShebangs docs/node_modules
    patchShebangs niljs/node_modules
    (cd smart-contracts; npm run build)
    (cd niljs; npm run build)

    cd clijs
    npm run bundle
  '';

  doCheck = enableTesting;

  checkPhase = ''
    export BIOME_BINARY=${biome}/bin/biome

    npm run lint

    ./dist/clijs | grep -q "The CLI tool for interacting with the =nil; cluster" || {
      echo "Error: Output does not contain the expected substring!" >&2
      exit 1
    }
    echo "smoke check passed"

    nohup nild run --http-port 8529 --collator-tick-ms=100 > nild.log 2>&1 & echo $! > nild_pid &

    npm run test:ci

    kill `cat nild_pid` && rm nild_pid

    echo "tests finished successfully"
  '';

  installPhase = ''
    mkdir -p $out
    mv ./dist/clijs $out/${pname}
  '';
}

