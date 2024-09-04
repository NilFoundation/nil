{
  description = "NIX dev env for Nil network";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    (flake-utils.lib.eachDefaultSystem (system:
      let
        revCount = self.revCount or self.dirtyRevCount or 1;
        rev = self.shortRev or self.dirtyShortRev or "unknown";
        version = "0.1.0-${toString revCount}";
        versionFull = "${version}-${rev}";
        pkgs = import nixpkgs { inherit system; };
      in
      rec {
        packages = rec {
          nil = (pkgs.callPackage ./nil.nix { buildGoModule = pkgs.buildGo123Module; });
          niljs = (pkgs.callPackage ./niljs.nix { });
          nildocs = (pkgs.callPackage ./nildocs.nix { nil = nil; enableTesting = true; });
          nilhardhat = (pkgs.callPackage ./nilhardhat.nix { });
          default = nil;
          formatters = (pkgs.callPackage ./formatters.nix { });
          nilcli = (pkgs.callPackage ./nilcli.nix { nil = nil; versionFull = versionFull; });
          nilsmartcontracts = (pkgs.callPackage ./nilsmartcontracts.nix { });
        };
        checks = rec {
          nil = (pkgs.callPackage ./nil.nix {
            buildGoModule = pkgs.buildGo123Module;
            enableRaceDetector = true;
            enableTesting = true;
          });
          niljs = (pkgs.callPackage ./niljs.nix {
            nil = nil;
            enableTesting = true;
          });
          nildocs = (pkgs.callPackage ./nildocs.nix {
            nil = nil;
            enableTesting = true;
          });
          nilhardhat = (pkgs.callPackage ./nilhardhat.nix {
            nil = nil;
            enableTesting = true;
          });
          nilsmartcontracts = (pkgs.callPackage ./nilsmartcontracts.nix {
            nil = nil;
            enableTesting = true;
          });
          default = pkgs.symlinkJoin {
            name = "all";
            paths = [ nil niljs nildocs nilhardhat ];
          };
        };
        bundlers =
          rec {
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
                  bash ${./scripts/binary_patch_version.sh} ./usr/bin/nild ${versionFull}
                  bash ${./scripts/binary_patch_version.sh} ./usr/bin/nil ${versionFull}
                  ${pkgs.fpm}/bin/fpm -s dir -t deb --name ${pkg.pname} -v ${version} --deb-use-file-permissions usr
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
