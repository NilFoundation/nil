{ lib
, stdenv
, fetchFromGitHub
, fetchNpmDeps
, callPackage
, npmHooks
, nodejs
, nil
, enableTesting ? false
}:

stdenv.mkDerivation rec {
  name = "smart-contracts";
  pname = "smart-contracts";
  src = lib.sourceByRegex ./. [
    "package.json"
    "package-lock.json"
    "^smart-contracts(/.*)?$"
    "^nil(/.*)?$"
  ];

  npmDeps = (callPackage ./npmdeps.nix { });

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [
    nodejs
    npmHooks.npmConfigHook
    nil
  ];

  dontConfigure = true;

  buildPhase = ''
    cd smart-contracts
    npm run compile
  '';

  doCheck = enableTesting;

  checkPhase = ''
    dir1="./contracts"
    dir2="../nil/contracts/solidity"

    find $dir1 -maxdepth 1 -type f -exec basename {} \; | sort > dir1_files
    find $dir2 -maxdepth 1 -type f -exec basename {} \; | sort > dir2_files
    if ! cmp -s dir1_files dir2_files; then
      echo "Directories $dir1 and $dir2 do not contain the same files"
      exit 1
    fi
    while read file; do
      if ! cmp -s "$dir1/$file" "$dir2/$file"; then
        echo "File $file is not identical in $dir1 and $dir2"
        exit 1
      fi
    done < dir1_files
  '';

  installPhase = ''
    mkdir -p $out
    touch $out/dummy
  '';
}
