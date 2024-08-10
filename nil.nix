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

  src = lib.sourceByRegex ./. ["Makefile" "go.mod" "go.sum" "^nil(/.*)?$"];

  # to obtain run `nix build` with vendorHash = "";
  vendorHash = "sha256-nJ+r8hVYuyULtW6YOV5E77+MlB0cZZDKO919DB0nVk4=";
  hardeningDisable = [ "all" ];

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
    "-X ${packageName}/nil/common/version.gitCommit=${rev}"
    "-X ${packageName}/nil/common/version.gitTag=${version}"
    "-X ${packageName}/nil/common/version.gitRevision=${toString revCount}"
  ];

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
