{ stdenv, fetchFromGitHub, fetchNpmDeps, npmHooks, nodejs, nil }:

stdenv.mkDerivation rec {
  name = "nil-hardhat-plugin";
  src = ./hardhat-plugin;

  npmDeps = fetchNpmDeps {
    inherit src;
    hash = "sha256-+ufc0TIW68Rm4F/zSzcGZpKY8Yv5QY4HTrAaXFxSRlE=";
  };

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [
    nodejs
    npmHooks.npmConfigHook
    nil
  ];

  dontConfigure = true;

  buildPhase = ''
    echo "run npm test in build phase when hardhat tests are ready"
  '';

  installPhase = ''
    mkdir -p $out
    touch $out/dummy
  '';
}
