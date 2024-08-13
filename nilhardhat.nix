{ lib, stdenv, fetchFromGitHub, fetchNpmDeps, npmHooks, nodejs, nil }:

stdenv.mkDerivation rec {
  name = "nil-hardhat-plugin";
  src = lib.sourceByRegex ./. ["package.json" "package-lock.json" "^hardhat-plugin(/.*)?$" "^niljs(/.*)?$"];

  npmDeps = fetchNpmDeps {
    inherit src;
    hash = "sha256-sRoOnUpqeCC9bdwmlxGor5WZrqzMVCV8wbyMcTIKbXg=";
  };

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [
    nodejs
    npmHooks.npmConfigHook
    nil
  ];

  dontConfigure = true;

  buildPhase = ''
    (cd niljs; npm run build)
    cd hardhat-plugin
    npm run build
    # uncomment when tests are fixed
    # npm test
  '';

  installPhase = ''
    mkdir -p $out
    touch $out/dummy
  '';
}
