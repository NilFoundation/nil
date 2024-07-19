{ stdenv, fetchFromGitHub, fetchNpmDeps, npmHooks, nodejs, nil }:

stdenv.mkDerivation rec {
  name = "nil-hardhat-plugin";
  src = fetchFromGitHub {
    owner = "NilFoundation";
    repo = "nil-hardhat-plugin";
    rev = "012aa3abeda9365233a12fb33b37bb70e6a245a6";
    sha256 = "sha256-18cSogr7h86KgOyaCfJU1KYrp0ugB7ENMJrGBTiOdeg=";
  };

  npmDeps = fetchNpmDeps {
    inherit src;
    hash = "sha256-PkAZGrbhAqkZAz5oggIrkdTExXUatLGS6dcJGoMbMcc=";
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
