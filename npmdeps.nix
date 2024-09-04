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
      ./smart-contracts/package.json
    ];
  };
  hash = "sha256-swm3hgJ3VMLokMTXXQEdGQwOMukOx69lXVeesxNt2wQ=";
})
