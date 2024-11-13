{
  description = "NIX dev env for Nil network";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    nil-released = {
      url = "github:NilFoundation/nil?rev=8f57aa19f88af84bb14a640a4c571c0f1610a2af";
      inputs = {
        nixpkgs.follows = "nixpkgs";
        flake-utils.follows = "flake-utils";
      };
    };
  };

  outputs = { self, nixpkgs, flake-utils, nil-released }:
    (flake-utils.lib.eachDefaultSystem (system:
      let
        revCount = self.revCount or self.dirtyRevCount or 1;
        rev = self.shortRev or self.dirtyShortRev or "unknown";
        version =
          if self.ref == "refs/heads/main" then
            "0.1.${toString revCount}-${rev}"
          else
            "0.0.${toString revCount}-${rev}";
        versionFull = "${version}";
        pkgs = import nixpkgs { inherit system; };
      in
      rec {
        packages = rec {
          nil = (pkgs.callPackage ./nil.nix { });
          niljs = (pkgs.callPackage ./niljs.nix { enableTesting = true; nil = nil; });
          nildocs = (pkgs.callPackage ./nildocs.nix { nil = nil; });
          nilhardhat = (pkgs.callPackage ./nilhardhat.nix { });
          default = nil;
          formatters = (pkgs.callPackage ./formatters.nix { });
          update_public_repo = (pkgs.callPackage ./update_public_repo.nix { });
          nilcli = (pkgs.callPackage ./nilcli.nix { nil = nil; versionFull = versionFull; });
          nilsmartcontracts = (pkgs.callPackage ./nilsmartcontracts.nix { });
          nilexplorer = (pkgs.callPackage ./nilexplorer.nix { });
        };
        checks = rec {
          nil = (pkgs.callPackage ./nil.nix {
            enableRaceDetector = true;
            enableTesting = true;
          });
          niljs = (pkgs.callPackage ./niljs.nix {
            nil = packages.nil;
            enableTesting = true;
          });
          nildocs = (pkgs.callPackage ./nildocs.nix {
            nil = packages.nil;
            enableTesting = true;
          });
          nilhardhat = (pkgs.callPackage ./nilhardhat.nix {
            nil = packages.nil;
            enableTesting = true;
          });
          nilexplorer = (pkgs.callPackage ./nilexplorer.nix {
            enableTesting = true;
          });
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
                  bash ${./scripts/binary_patch_version.sh} ./usr/bin/cometa ${versionFull}
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
