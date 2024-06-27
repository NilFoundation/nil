{
  description = "NIX dev env for Nil network";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    (flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
        };

      in rec {
        packages = rec {
          nil = (pkgs.callPackage ./nil.nix { src_repo = self; });
          default = nil;
        };
        checks = rec {
          nil = (pkgs.callPackage ./nil.nix { src_repo = self; enableRaceDetector = true; enableTesting = true; });
          default = nil;
        };

        bundlers = rec {
          deb = pkg: pkgs.stdenv.mkDerivation {
            name = "deb-package-${pkg.pname}";
            buildInputs = [
              pkgs.fpm
            ];

            unpackPhase = "true";
            buildPhase = ''
              export HOME=$PWD
              mkdir -p ./usr
              cp -r ${pkg}/bin ./usr/
              chmod -R u+rw,g+r,o+r ./usr
              chmod -R u+rwx,g+rx,o+rx ./usr/bin
              ${pkgs.fpm}/bin/fpm -s dir -t deb --name ${pkg.pname} -v ${pkg.version} --deb-use-file-permissions usr
            '';
            installPhase = ''
              mkdir -p $out
              cp -r *.deb $out
            '';
          };
          default = deb;
        };
      })
    );
}
