{
  description = "NIX dev env for toy version of nil network";

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
          nil = pkgs.pkgsStatic.buildGoModule rec {
            name = "nil";
            pname = "nil";
            revCount = self.revCount or self.dirtyRevCount or 1;
            version = "0.1.0-${toString revCount}";

            preBuild = ''
                make compile-contracts
            '';

            src = ./.;
            # to obtain run `nix build` with vendorHash = "";
            vendorHash = "sha256-OEmpac6TKHRfXXAr2VlzDX0/f3MC6qci3Sd0Etx5n9E=";
            hardeningDisable = [ "all" ];
            ldflags = [
              "-linkmode external"
            ];

            nativeBuildInputs = [
              pkgs.solc
            ];

            doCheck = true;
            checkFlags = ["-race" "-tags assert"];
          };

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

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go_1_22
            gotools
            go-tools
            gopls
            golangci-lint
            gofumpt
            gci
            delve
            solc
          ];

          hardeningDisable = [ "all" ];

          shellHook = ''
            export GO_CFG_DIR=$HOME/.nix/go/$(go env GOVERSION)

            mkdir -p $GO_CFG_DIR/config $GO_CFG_DIR/cache $GO_CFG_DIR/pkg/mod

            export GOENV="$GO_CFG_DIR/config/env"

            go env -w GOCACHE="$GO_CFG_DIR/cache"
            go env -w GOMODCACHE="$GO_CFG_DIR/pkg/mod"
          '';
        };

        overlays.default = final: prev: {
          nil = packages.default;
        };
      })
    );
}
