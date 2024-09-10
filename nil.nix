{ lib
, stdenv
, buildGoModule
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
}:
let inherit (lib) optional;
  overrideBuildGoModule = pkg: pkg.override { buildGoModule = buildGoModule; };
in
buildGoModule rec {
  name = "nil";
  pname = "nil";

  preBuild = ''
    make compile-contracts ssz rpcspec
  '';

  src = lib.sourceByRegex ./. [ "Makefile" "go.mod" "go.sum" "^nil(/.*)?$" "^smart-contracts(/.*)?$" ];

  # to obtain run `nix build` with vendorHash = "";
  vendorHash = "sha256-M8+B5IPXaXjQWfXOudfqSiy/SjjVTYZ8TgmM/4Emeu8=";
  hardeningDisable = [ "all" ];

  postInstall = ''
    mkdir -p $out/share/doc/nil
    cp openrpc.json $out/share/doc/nil
  '';

  CGO_ENABLED = if enableRaceDetector then 1 else 0;

  nativeBuildInputs = [
    solc
    clickhouse
    (overrideBuildGoModule gotools)
    (overrideBuildGoModule go-tools)
    (overrideBuildGoModule gopls)
    (overrideBuildGoModule golangci-lint)
    (overrideBuildGoModule gofumpt)
    (overrideBuildGoModule gci)
    (overrideBuildGoModule delve)
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
