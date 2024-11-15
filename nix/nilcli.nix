{ lib
, stdenv
, versionFull
, nil
}:

stdenv.mkDerivation rec {
  name = "nilcli";
  pname = "nilcli";

  src = ../scripts;

  nativeBuildInputs = [
    nil
  ];

  dontConfigure = true;

  buildPhase = ''
  '';

  installPhase = ''
    mkdir -p $out/bin
    cp -r ${nil}/bin/nil $out/bin
    bash binary_patch_version.sh $out/bin/nil ${versionFull}
  '';
}
