{
  description = "NIX dev env for Nil network";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    nil-released = {
      url =
        "github:NilFoundation/nil?rev=8f57aa19f88af84bb14a640a4c571c0f1610a2af";
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
        version = "0.1.5-${toString revCount}";
        versionFull = "${version}-${rev}";
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ (import ./nix/overlay.nix) ];
        };
      in
      rec {
        packages = rec {
          solc = (pkgs.callPackage ./nix/solc.nix { });
          nil = (pkgs.callPackage ./nix/nil.nix { solc = solc; });
          niljs = (pkgs.callPackage ./nix/niljs.nix { solc = solc; });
          clijs = (pkgs.callPackage ./nix/clijs.nix { nil = nil; });
          nildocs = (pkgs.callPackage ./nix/nildocs.nix {
            nil = nil;
            solc = solc;
          });
          default = nil;
          formatters = (pkgs.callPackage ./nix/formatters.nix { });
          update_public_repo =
            (pkgs.callPackage ./nix/update_public_repo.nix { });
          nilcli = (pkgs.callPackage ./nix/nilcli.nix {
            nil = nil;
            versionFull = versionFull;
          });
          nilsmartcontracts =
            (pkgs.callPackage ./nix/nilsmartcontracts.nix { });
          nilexplorer = (pkgs.callPackage ./nix/nilexplorer.nix { });
          walletextension = (pkgs.callPackage ./nix/walletextension.nix { });
          uniswap = (pkgs.callPackage ./nix/uniswap.nix { });
          docsaibackend = (pkgs.callPackage ./nix/docsaibackend.nix { });
          rollup-bridge-contracts =
            (pkgs.callPackage ./nix/rollup-bridge-contracts.nix { });
        };
        checks = rec {
          nil = (pkgs.callPackage ./nix/nil.nix {
            enableRaceDetector = true;
            enableTesting = true;
            solc = packages.solc;
          });

          # split tests into groups
          ibft = nil.override {
            testGroup = "ibft";
            parallelTesting = true;
          };
          heavy = nil.override {
            testGroup = "heavy";
            parallelTesting = true;
          };
          others = nil.override {
            testGroup = "others";
            parallelTesting = true;
          };

          niljs = (pkgs.callPackage ./nix/niljs.nix {
            nil = packages.nil;
            solc = packages.solc;
            enableTesting = true;
          });
          clijs = (pkgs.callPackage ./nix/clijs.nix {
            nil = packages.nil;
            enableTesting = true;
          });
          nildocs = (pkgs.callPackage ./nix/nildocs.nix {
            nil = packages.nil;
            enableTesting = true;
            solc = packages.solc;
          });
          nilexplorer = (pkgs.callPackage ./nix/nilexplorer.nix {
            enableTesting = true;
            nil = packages.nil;
          });
          walletextension = (pkgs.callPackage ./nix/walletextension.nix {
            nil = packages.nil;
            enableTesting = true;
          });
          uniswap = (pkgs.callPackage ./nix/uniswap.nix {
            nil = packages.nil;
            enableTesting = true;
          });
          rollup-bridge-contracts =
            (pkgs.callPackage ./nix/rollup-bridge-contracts.nix {
              enableTesting = true;
            });
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
                mkdir -p ./usr/share/${packages.nilexplorer.name}
                mkdir -p ./usr/share/${packages.docsaibackend.name}
                mkdir -p ./usr/share/${packages.rollup-bridge-contracts.name}

                cp -r ${pkg}/bin ./usr/
                cp -r ${pkg}/share ./usr/
                cp -r ${packages.nildocs.outPath}/* ./usr/share/${packages.nildocs.pname}
                cp -r ${packages.nilexplorer.outPath}/* ./usr/share/${packages.nilexplorer.name}
                cp -r ${packages.docsaibackend.outPath}/* ./usr/share/${packages.docsaibackend.name}
                cp -r ${packages.rollup-bridge-contracts.outPath}/{.,}* ./usr/share/${packages.rollup-bridge-contracts.name}

                chmod -R u+rw,g+r,o+r ./usr
                chmod -R u+rwx,g+rx,o+rx ./usr/bin
                chmod -R u+rwx,g+rx,o+rx ./usr/share/${packages.nildocs.pname}
                chmod -R u+rwx,g+rx,o+rx ./usr/share/${packages.nilexplorer.name}
                chmod -R u+rwx,g+rx,o+rx ./usr/share/${packages.docsaibackend.name}

                bash ${
                  ./scripts/binary_patch_version.sh
                } ./usr/bin/nild ${versionFull}
                bash ${
                  ./scripts/binary_patch_version.sh
                } ./usr/bin/nil ${versionFull}
                bash ${
                  ./scripts/binary_patch_version.sh
                } ./usr/bin/cometa ${versionFull}
                ${pkgs.fpm}/bin/fpm -s dir -t deb --name ${pkg.pname} -v ${version} --deb-compression xz --deb-use-file-permissions usr
              '';
              installPhase = ''
                mkdir -p $out
                cp -r *.deb $out
              '';
            };
          default = deb;
        };
      }))

    // {

      nixosConfigurations.container = nixpkgs.lib.nixosSystem {
        system = "x86_64-linux";
        modules = [ ./nix/container.nix ];

      };
    };
}
