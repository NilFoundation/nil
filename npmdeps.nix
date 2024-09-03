{ lib, stdenv, fetchNpmDeps }:
let
  inherit (lib) fileset;
in
(fetchNpmDeps {
  src = fileset.toSource {
    root = ./.;
    fileset = fileset.unions [
      ./package-lock.json
      ./package.json
      ./docs/package.json
      ./hardhat-plugin/package.json
      ./niljs/package.json
    ];
  };
  hash = "sha256-ZmubuxjnM5HrbFWmdpEnX1cYXXGEq1yG0ZHuzkHOBrE=";
})
