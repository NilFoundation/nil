{ lib
, stdenv
, callPackage
, nil
, pkgs
, rollup-bridge-contracts
, enableTesting ? false
}:

stdenv.mkDerivation rec {
  name = "bridge-test";
  pname = "bridge-test";
  src = lib.sourceByRegex ./.. [
    "^rollup-bridge-contracts(/.*)?$"
    "^nil(/.*)?$"
  ];

  buildPhase = "";

  doCheck = enableTesting;
  checkPhase = ''
    export NILD_BIN=nild
    export FAUCET_BIN=faucet
    export RELAYER_BIN=relayer
    export GETH_BIN=${lib.getExe pkgs.go-ethereum}

    cd rollup-bridge-contracts
    echo "Running bridge integration tests"
    ./test_integration.sh
    echo "Bridge integration tests finished"
  '';
}
