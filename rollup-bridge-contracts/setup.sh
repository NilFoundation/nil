#!/bin/bash

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Install npm dependencies
echo "Installing npm dependencies..."
npm install

# Run Hardhat clean and compile
echo "Running Hardhat clean and compile..."
npx hardhat clean
npx hardhat compile

# Install Forge dependencies
if command_exists forge; then
    echo "Installing Forge dependencies..."
    forge install foundry-rs/forge-std --no-commit
    forge install transmissions11/solmate --no-commit
    forge install dapphub/ds-test --no-commit

    echo "Running Forge clean and compile..."
    forge clean
    forge build

    echo "Running Forge tests..."
    forge test
else
    echo "Forge is not installed. Skipping Forge steps."
fi

# Health check
echo "Performing health check..."
if npx hardhat compile && forge build && forge test; then
    echo "Setup completed successfully!"
else
    echo "Setup failed. Please check the errors above."
    exit 1
fi