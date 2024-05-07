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
        packages = {
          default = pkgs.buildGoModule {
            name = "nil";
            src = ./.;
            # to obtain run `nix build` with vendorHash = "";
            vendorHash = "sha256-Z03rBTHuEvwyn3HzIGXXuQm+uww/igafoqQ9dEvWzSM=";
            hardeningDisable = [ "all" ];

            doCheck = true;
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gotools
            go-tools
            gopls
            golangci-lint
            delve
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
