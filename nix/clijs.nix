{ pkgs
, lib
, stdenv
, callPackage
, npmHooks
}:

stdenv.mkDerivation rec {
  name = "clijs";
  pname = "clijs";
  src = lib.sourceByRegex ./.. [
    "package.json"
    "package-lock.json"
    "^clijs(/.*)?$"
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

  # See https://nodejs.org/api/single-executable-applications.html
  buildPhase = ''
    cd clijs

    # Generate sea-prep.blob
    ${pkgs.pkgsStatic.nodejs_22}/bin/node --experimental-sea-config ./sea-config.json

    # Copy node executable
    cp ${pkgs.pkgsStatic.nodejs_22}/bin/node hello
    chmod 755 hello

    # Create executable
    npx postject \
      hello NODE_SEA_BLOB sea-prep.blob \
      --sentinel-fuse NODE_SEA_FUSE_fce680ab2cc467b6e072b8b5df1996b2
  '';

  installPhase = ''
    mkdir -p $out
    mv hello $out/hello
  '';
}

