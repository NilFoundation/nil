{ stdenv, fetchFromGitHub, fetchNpmDeps, npmHooks, nodejs, nil }:

stdenv.mkDerivation rec {
  name = "nil-hardhat-plugin";
  src = ./hardhat-plugin;

  npmDeps = fetchNpmDeps {
    inherit src;
    hash = "sha256-8X/Q45kueTJN9S+6nC5QJV6ROnxUaUbrZDepb0WTdy8=";
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
    npm link ${niljs}
    npm run build
    npm test
  '';

  installPhase = ''
    mkdir -p $out
    touch $out/dummy
  '';
}
