{ pkgs
, lib
, stdenv
, callPackage
, npmHooks
, enableTesting ? false
}:

stdenv.mkDerivation rec {
  name = "clijs";
  pname = "clijs";
  src = lib.sourceByRegex ./.. [
    "package.json"
    "package-lock.json"
    "^clijs(/.*)?$"
    "^niljs(/.*)?$"
    "^smart-contracts(/.*)?$"
  ];

  npmDeps = (callPackage ./npmdeps.nix { });

  NODE_PATH = "$npmDeps";

  nativeBuildInputs = [
    pkgs.pkgsStatic.nodejs_22
    npmHooks.npmConfigHook
  ];

  dontConfigure = true;

  preUnpack = ''
    echo "Setting UV_USE_IO_URING=0 to work around the io_uring kernel bug"
    export UV_USE_IO_URING=0
  '';

  buildPhase = ''
    patchShebangs docs/node_modules
    patchShebangs niljs/node_modules
    (cd smart-contracts; npm run build)
    (cd niljs; npm run build)

    cd clijs
    npm run bundle

    # See https://nodejs.org/api/single-executable-applications.html
    # Generate sea-prep.blob
    ${pkgs.pkgsStatic.nodejs_22}/bin/node --experimental-sea-config ./sea-config.json

    # Copy node executable
    cp ${pkgs.pkgsStatic.nodejs_22}/bin/node clijs
    chmod 755 clijs

    # Create executable
    ${pkgs.pkgsStatic.nodejs_22}/bin/npx postject \
      clijs NODE_SEA_BLOB sea-prep.blob \
      --sentinel-fuse NODE_SEA_FUSE_fce680ab2cc467b6e072b8b5df1996b2
  '';

  doCheck = enableTesting;

  checkPhase = ''
    ./clijs | grep -q "The CLI tool for interacting with the =nil; cluster" || {
      echo "Error: Output does not contain the expected substring!" >&2
      exit 1
    }
    echo "Smoke check passed!"
  '';

  installPhase = ''
    mkdir -p $out
    mv clijs $out/${pname}
  '';
}

