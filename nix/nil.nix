{ lib
, stdenv
, buildGo123Module
, enableRaceDetector ? false
, enableTesting ? false
, jq
, solc
, solc-select
, clickhouse
, go-tools
, gotools
, golangci-lint
, gofumpt
, gci
, delve
, gopls
, protoc-gen-go
, protobuf
}:
let inherit (lib) optional;
  overrideBuildGoModule = pkg: pkg.override { buildGoModule = buildGo123Module; };
in
buildGo123Module rec {
  name = "nil";
  pname = "nil";

  preBuild = ''
    make generated rpcspec
    export HOME="$TMPDIR"
    mkdir -p ~/.gsolc-select/artifacts/solc-0.8.28
    ln -f -s ${solc}/bin/solc ~/.gsolc-select/artifacts/solc-0.8.28/solc-0.8.28
  '';

  src = lib.sourceByRegex ./.. [
    "Makefile"
    "go.mod"
    "go.sum"
    "^nil(/.*)?$"
    "^smart-contracts(/.*)?$"
    "^uniswap(/.*)?$"
  ];

  # to obtain run `nix build` with vendorHash = "";
  vendorHash = "sha256-1EZJmurQ1iZREUdWAQmO/vrkHtOphOo64kIbOd7yLJQ=";
  hardeningDisable = [ "all" ];

  postInstall = ''
    mkdir -p $out/share/doc/nil
    cp openrpc.json $out/share/doc/nil
  '';

  env.CGO_ENABLED = if enableRaceDetector then 1 else 0;

  nativeBuildInputs = [
    jq
    solc
    solc-select
    clickhouse
    protobuf
    (overrideBuildGoModule gotools)
    (overrideBuildGoModule go-tools)
    (overrideBuildGoModule gopls)
    golangci-lint
    (overrideBuildGoModule gofumpt)
    (overrideBuildGoModule gci)
    (overrideBuildGoModule delve)
    (overrideBuildGoModule protoc-gen-go)
  ];

  packageName = "github.com/NilFoundation/nil";

  doCheck = enableTesting;
  checkFlags = [ "-tags assert,test" ]
    ++ (if enableRaceDetector then [ "-race" ] else [ ]);

  GOFLAGS = [ "-modcacherw" ];
  shellHook = ''
    eval "$configurePhase"
    export GOCACHE=/tmp/${vendorHash}/go-cache
    export GOMODCACHE=/tmp/${vendorHash}/go/mod/cache
    chmod -R u+w vendor
    mkdir -p ~/.solc-select/artifacts/solc-0.8.28
    ln -f -s ${solc}/bin/solc ~/.solc-select/artifacts/solc-0.8.28/solc-0.8.28
  '';
}
