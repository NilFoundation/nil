{ lib
, gccStdenv
, fetchurl
, pkgs
}:

let
  pname = "zeth";
  version = "0.1.0";
  meta = with lib; {
    description = "Reference tool for rexecute blocks";
    homepage = "https://github.com/akokoshn/zeth";
    # license = licenses.apache;
  };

  zeth =
    gccStdenv.mkDerivation
      rec {
        inherit pname version meta;

        executable = fetchurl {
          url = "https://github.com/akokoshn/zeth/releases/download/dev/zeth-ethereum";
          sha256 = "sha256-HXbw7RfiTR4+CMdryWvR/0YpaLK8fsccSxameBkJqIk=";
        };

        phases = [ "installPhase" ];

        installPhase = ''
          echo "Install Zeth"
          mkdir -p $out/bin
          cp ${executable} $out/bin/zeth-ethereum
          chmod 777 $out/bin/zeth-ethereum
        '';
      };
in
zeth
