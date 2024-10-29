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
  hash = "sha256-xmnfxu4YOIXM5//ajy1YpzmL1SIwo6/33ZjqKgwGIiQ=";
})
