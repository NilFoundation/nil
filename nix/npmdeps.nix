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
      ../docs/package.json
      ../hardhat-plugin/package.json
      ../niljs/package.json
      ../create-nil-hardhat-project/package.json
      ../smart-contracts/package.json
      ../explorer_backend/package.json
      ../explorer_frontend/package.json
      ../uniswap/package.json
    ];
  };
  hash = "sha256-w09TbVQlp6W1xu4NseSyLO4GFV9pHBhsICZhWBldlpk=";
})
