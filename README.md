# Simple cluster

This is a simple implementation of the cluster with ethereum-like data structures.


## Environment setup

You'd need Nix to be installed.
Afterwards, do:

```
nix develop
```

And you'd be in the environment with everything necessary already set up.

P.S. If you don't have Nix on silicon mac you should use the following command to install it:

```
curl --proto '=https' --tlsv1.2 -sSf -L https://install.determinate.systems/nix | sh -s -- install
```

Official Nix installation for macOS is not supported on silicon macs in some cases.

## Building

To build everything, type:

```
make
```

## Running tests

Type:

```
make test
```

## Generate SSZ serialization code

To generate SSZ serialization code, type:

```
make ssz
```

## Linting

```
make lint
```

Linters are configured in `.golangci.yml`. Docs: https://golangci-lint.run/usage/linters.

How to integrate linters with your IDE:

https://github.com/mvdan/gofumpt?tab=readme-ov-file#installation

https://golangci-lint.run/welcome/integrations/

https://github.com/luw2007/gci?tab=readme-ov-file#installation

## Packaging

To create a platform-agnostic deb package:

```
nix bundle --bundler . .#nil
```
