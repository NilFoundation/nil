{ lib
, stdenv
, buildGo123Module
, enableRaceDetector ? false
, enableTesting ? false
, solc
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
  '';

  src = lib.sourceByRegex ./. [ "Makefile" "go.mod" "go.sum" "^nil(/.*)?$" "^smart-contracts(/.*)?$" ];

  # to obtain run `nix build` with vendorHash = "";
  vendorHash = "sha256-+gRif5x8M0qbBix+2dyO6zwYh9RUNtwpXnOyyR0YyL0=";
  hardeningDisable = [ "all" ];

  postInstall = ''
    mkdir -p $out/share/doc/nil
    cp openrpc.json $out/share/doc/nil
  '';

  CGO_ENABLED = if enableRaceDetector then 1 else 0;

  nativeBuildInputs = [
    solc
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
  '';
}
