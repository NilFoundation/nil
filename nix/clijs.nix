{ pkgs
, lib
, buildGo124Module
, stdenv
, biome
, jq
, moreutils
, callPackage
, pnpm_10
, nil
, solc
, gotools
, protobuf
, protoc-gen-go
, python3
, enableTesting ? false
}:

let
  sigtool = callPackage ./sigtool.nix { };
  nodejs_static = pkgs.pkgsStatic.nodejs_22;
  pnpm_static = (pnpm_10.override { nodejs = nodejs_static; });
  overrideBuildGoModule = pkg: pkg.override { buildGoModule = buildGo124Module; };
in
stdenv.mkDerivation rec {
  name = "clijs";
  pname = "clijs";
  src = lib.sourceByRegex ./.. [
    ".oclifrc.json"
    "package.json"
    "pnpm-workspace.yaml"
    "pnpm-lock.yaml"
    ".npmrc"
    "^clijs(/.*)?$"
    "^niljs(/.*)?$"
    "^smart-contracts(/.*)?$"
    "biome.json"

    "Makefile"
    "go.mod"
    "go.sum"
    "^nil(/.*)?$"
  ];
  pnpmDeps = (callPackage ./npmdeps.nix { });

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [
    nodejs_static
    pnpm_static.configHook
    biome
    jq
    moreutils
    solc
    protobuf
    (overrideBuildGoModule gotools)
    (overrideBuildGoModule protoc-gen-go)
    (python3.withPackages (ps: with ps; [
      safe-pysha3
    ]))
  ] ++ lib.optionals stdenv.buildPlatform.isDarwin [ sigtool ]
    ++ (if enableTesting then [ nil ] else [ ]);

  preUnpack = ''
    echo "Setting UV_USE_IO_URING=0 to work around the io_uring kernel bug"
    export UV_USE_IO_URING=0
  '';

  postUnpack = ''
    mkdir -p source/nil
    cp -R ${nil}/* source/nil
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

    ./dist/clijs util list-commands > bundled_cli_commands

    pnpm run build-to dist-tmp
    jq '.commands = "./dist-tmp/src/commands"' .oclifrc.json | sponge .oclifrc.json
    node ./bin/run.js util list-commands > non_bundled_cli_commands

    diff bundled_cli_commands non_bundled_cli_commands > /dev/null || {
      echo "have you added new command to the `sea.ts` file?"
      echo "bundlied cli command list:"
      cat bundled_cli_commands
      echo "non-bundlied cli command list:"
      cat non_bundled_cli_commands
      rm bundled_cli_commands non_bundled_cli_commands
      exit 1
    }

    echo "smoke check passed"

    echo "running js tests"

    env NILD=nild pnpm run test:ci || {
      echo "tests failed. nild.log:"
      cat nild.log
      exit 1
    }

    echo "running golang tests"
    cd ..

    export PATH="${nil.go}/bin:$PATH"
    make -j$NIX_BUILD_CORES rlp pb solidity_console generate_mocks

    testPkgs=$(go list "./nil/tests/..." | grep "tests/cli" | sed -e 's,^[.]/,,' | LC_ALL=C sort -u)
    for pkg in $testPkgs; do
      echo "testing $pkg"
      go test ${(builtins.concatStringsSep " " nil.checkFlags)} $pkg
    done

    echo "tests finished successfully"
  '';

  installPhase = ''
    mkdir -p $out
    mv ./clijs/dist/clijs $out/${pname}
  '';
}
