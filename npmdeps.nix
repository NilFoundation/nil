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
  hash = "sha256-ZEetiy0kup6JDCnRcl63iRddH81mMk7ue+3vCOZNk6Q=";
})
