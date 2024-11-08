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
      ./create-nil-hardhat-project/package.json
      ./smart-contracts/package.json
      ./explorer_backend/package.json
    ];
  };
  hash = "sha256-YXZcaUZm5lg3u4f41Bj39c976kv8P+rzMQMTTLEfZw8=";
})
