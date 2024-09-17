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
      ./hardhat-examples/package.json
      ./smart-contracts/package.json
    ];
  };
  hash = "sha256-0u/1hvEBTSWqnjo1t22GFQAH+ihksDJq3se9Oo0KsZs=";
})
