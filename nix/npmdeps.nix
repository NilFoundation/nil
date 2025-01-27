{ lib, stdenv, fetchNpmDeps }:
let
  inherit (lib) fileset;
in
(fetchNpmDeps {
  src = fileset.toSource {
    root = ./..;
    fileset = fileset.unions [
      ../package-lock.json
      ../package.json
      ../clijs/package.json
      ../docs/package.json
      ../niljs/package.json
      ../create-nil-hardhat-project/package.json
      ../smart-contracts/package.json
      ../explorer_backend/package.json
      ../explorer_frontend/package.json
      ../uniswap/package.json
    ];
  };
<<<<<<< HEAD
  hash = "sha256-P0bwGmK5nhqVA9k4zIOsREyCrhkllxV2jH9zoOBPMtE=";
=======
  hash = "sha256-aBND35jj4t3dz0237uCBa/1DIRSIDNDZb/gHS9Y303M=";
>>>>>>> 33f46b7 (prepared docs for the 28/01 release)
})
