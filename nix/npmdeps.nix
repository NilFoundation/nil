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
    ];
  };
  hash = "sha256-5/s/axw15BbBQTnhcR+h5nYXQPDr3CZ1xK0XuE9TTMU=";
})
