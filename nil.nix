{ lib, stdenv, src_repo, buildGoModule, enableRaceDetector ? false
, enableTesting ? false, solc, clickhouse, go-tools, gotools, golangci-lint
, gofumpt, gci, delve, gopls }:
let inherit (lib) optional;
in buildGoModule rec {
  name = "nil";
  pname = "nil";
  revCount = src_repo.revCount or src_repo.dirtyRevCount or 1;
  version = "0.1.0-${toString revCount}";

  preBuild = ''
    make compile-contracts ssz rpcspec
  '';

  src = src_repo;
  # to obtain run `nix build` with vendorHash = "";
  vendorHash = "sha256-MrwLqrmnOYUp6y/zg81q06gd8An3Aau4gFckYNbislY=";
  hardeningDisable = [ "all" ];

  postConfigure = ''
    export GOCACHE=/tmp/${vendorHash}/go-cache
    export GOMODCACHE=/tmp/${vendorHash}/go/mod/cache
  '';

  postInstall = ''
    mkdir -p $out/share/doc/nil
    cp openrpc.json $out/share/doc/nil
  '';

  CGO_ENABLED = if enableRaceDetector then 1 else 0;

  nativeBuildInputs = [
    solc
    clickhouse
    gotools
    go-tools
    gopls
    golangci-lint
    gofumpt
    gci
    delve
  ];

  rev = src_repo.shortRev or src_repo.dirtyShortRev or "unknown";
  packageName = "github.com/NilFoundation/nil";
  ldflags = [
    "-X ${packageName}/cmd/nil_cli/version.gitCommit=${rev}"
    "-X ${packageName}/cmd/nil_cli/version.gitTag=${version}"
  ];

  doCheck = enableTesting;
  checkFlags = [ "-tags assert,test" ]
    ++ (if enableRaceDetector then [ "-race" ] else [ ]);

  GOFLAGS = [ "-modcacherw" ];
  shellHook = ''
    eval "$configurePhase"
    chmod -R u+w vendor
  '';
}
