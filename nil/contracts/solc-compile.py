#!/usr/bin/env python3

import json
import subprocess
import argparse
import solc_select.solc_select as ss

# Compile a Solidity contract with json compiler task used in the Cometa service.

def main():
    parser = argparse.ArgumentParser(description="Compile a Solidity contract with json compiler task.")
    parser.add_argument("-f", "--file", required=True, help="Path to the Solidity contract file.")
    parser.add_argument("-c", "--compiler-json", required=True, help="Path to the JSON file with compiler task.")
    parser.add_argument("-o", "--output-dir", required=True, help="Output directory for compiled contract files.")
    parser.add_argument("-v", "--verbose", action='store_true', help="Print verbose logs.")

    args = parser.parse_args()

    # Load compiler configuration
    with open(args.compiler_json, 'r') as f:
        config = json.load(f)

    # Switch solc version
    ss.switch_global_version(config.get("compilerVersion"), True)

    opt_args = []
    optimize_flag = config.get("settings", {}).get("optimizer", {}).get("enabled")
    if optimize_flag:
        opt_args += "--optimize"
        optimize_runs = config.get("settings", {}).get("optimizer", {}).get("runs")
        if optimize_runs:
            opt_args += "--optimize-runs"
            opt_args += str(optimize_runs)

    # Compile the contract
    compile_command = [
        "solc", "--overwrite", "--metadata-hash", "none", "--no-cbor-metadata", "--abi", "--bin",
        "-o", args.output_dir, args.file, *opt_args
    ]

    if args.verbose:
        print(f"Solc command: {' '.join(compile_command)}")

    try:
        out = subprocess.run(compile_command, check=True, capture_output=True, text=True).stdout
        print(out, end='')
    except subprocess.CalledProcessError as e:
        print(f"Error compiling contract: {e.stderr}", end='')
        exit(1)

if __name__ == "__main__":
    main()
