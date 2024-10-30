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
      ./explorer_backend/package.json
    ];
  };
  hash = "sha256-o9F96L+SG39/hX3aoD5EZwp5adqplOtZYVU1Zh+Dt/M=";
})
