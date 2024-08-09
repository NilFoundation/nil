{
  description = "NIX dev env for Nil network";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    (flake-utils.lib.eachDefaultSystem (system:
      let pkgs = import nixpkgs { inherit system; };
#      node-modules = pkgs.mkYarnPackage {
#          name = "node-modules";
#          src = ./docs;
#        };
#      docs = pkgs.stdenv.mkDerivation {
#          name = "nildocs";
#          src = ./docs;
#          buildInputs = [pkgs.yarn node-modules];
#          buildPhase = ''
#            ln -s ${node-modules}/node_modules ./docs/node_modules
#            ${pkgs.yarn}/bin/yarn build
#          '';
#          installPhase =  ''
#          mkdir $out
#          mv dist $out/lib
#          '';
#
#        };
      in rec {
        packages = rec {
          nil = (pkgs.callPackage ./nil.nix { src_repo = self; });
          niljs = (pkgs.callPackage ./niljs.nix { nil = nil; });
          nildocs = (pkgs.callPackage ./nildocs.nix { nil = nil; });
          nilhardhat = (pkgs.callPackage ./nilhardhat.nix { nil = nil; });
          default = nil;
        };
        checks = rec {
          nil = (pkgs.callPackage ./nil.nix {
            src_repo = self;
            enableRaceDetector = true;
            enableTesting = true;
          });
          niljs = (pkgs.callPackage ./niljs.nix { nil = nil; });
          nildocs = (pkgs.callPackage ./nildocs.nix { nil = nil; });
          nilhardhat = (pkgs.callPackage ./nilhardhat.nix { nil = nil; });
          default = pkgs.symlinkJoin {
            name = "all";
            paths = [ nil niljs nildocs nilhardhat ];
          };
        };
        bundlers = rec {
          deb = pkg:
            pkgs.stdenv.mkDerivation {
              name = "deb-package-${pkg.pname}";
              buildInputs = [ pkgs.fpm ];

              unpackPhase = "true";
              buildPhase = ''
                export HOME=$PWD
                mkdir -p ./usr
                mkdir -p ./usr/share/${packages.nildocs.pname}
                cp -r ${pkg}/bin ./usr/
                cp -r ${pkg}/share ./usr/
                cp -r ${packages.nildocs.outPath}/* ./usr/share/${packages.nildocs.pname}
                chmod -R u+rw,g+r,o+r ./usr
                chmod -R u+rwx,g+rx,o+rx ./usr/bin
                chmod -R u+rwx,g+rx,o+rx ./usr/share/${packages.nildocs.pname}
                ${pkgs.fpm}/bin/fpm -s dir -t deb --name ${pkg.pname} -v ${pkg.version} --deb-use-file-permissions usr
              '';
              installPhase = ''
                mkdir -p $out
                cp -r *.deb $out
              '';
            };
          default = deb;
        };
      }));
}
