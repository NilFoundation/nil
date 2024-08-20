{ stdenv
, pkgs
,
}:
stdenv.mkDerivation rec {
  pname = "formatters";

  version = "0.0.1";

  dontUnpack = true;

  propagatedBuildInputs = with pkgs; [
    nixpkgs-fmt
    shfmt
    util-linux
  ];
}
