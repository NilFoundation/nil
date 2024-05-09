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
