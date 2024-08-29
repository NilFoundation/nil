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
  hash = "sha256-b4fjrq6rhg+6PR0ZVjtMzSNIEsW8npjZ8yeThIcIxZk=";
})
