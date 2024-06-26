{ lib, stdenv, src_repo, buildGoModule, enableRaceDetector ? false, enableTesting ? false, solc }:
let inherit (lib) optional;
in buildGoModule rec {
  name = "nil";
  pname = "nil";
  revCount = src_repo.revCount or src_repo.dirtyRevCount or 1;
  version = "0.1.0-${toString revCount}";

  preBuild = ''
    make compile-contracts
  '';

  src = src_repo;
  # to obtain run `nix build` with vendorHash = "";
  vendorHash = "sha256-QlZZBBoCOYjDBeKaiX7Q3haJGc7cb+3oqHN7sOP8wxE=";
  hardeningDisable = [ "all" ];

  CGO_ENABLED = if enableRaceDetector then 1 else 0;

  nativeBuildInputs = [ solc ];

  doCheck = enableTesting;
  checkFlags = [ "-tags assert,test" ]
    ++ (if enableRaceDetector then [ "-race" ] else [ ]);
}
