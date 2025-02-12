# Stage 1: Build stage
FROM node:20.3.0 AS build

# Set the working directory
WORKDIR /app

# Copy package.json and package-lock.json
COPY package.json package-lock.json ./

# Install dependencies
RUN npm install

# Copy the deployment scripts and contract artifacts
COPY scripts/geth-ops/fund-wallet.ts /app/fund-wallet.ts
COPY wait-for-it.sh /app/wait-for-it.sh
COPY tsconfig.json /app/tsconfig.json

# Make the wait-for-it script executable
RUN chmod +x /app/wait-for-it.sh

# Stage 2: Final stage
FROM ethereum/client-go:latest

# Install wget for healthcheck
RUN apk add --no-cache wget

# Install Node.js and npm
RUN apk add --no-cache nodejs npm

# Install ts-node globally
RUN npm install -g ts-node typescript

# Install curl for healthcheck
RUN apk add --no-cache curl

# Copy the necessary files from the build stage
COPY --from=build /app /app
COPY --from=build /app/node_modules /app/node_modules

# Set the working directory
WORKDIR /app

# Start the Geth node
ENTRYPOINT ["geth", "--http.vhosts", "'*,localhost,host.docker.internal'", "--http", "--http.api", "admin,debug,web3,eth,txpool,miner,net,dev,personal", "--http.corsdomain", "*", "--http.addr", "0.0.0.0", "--nodiscover", "--maxpeers", "0", "--mine", "--networkid", "1337", "--dev", "--allow-insecure-unlock", "--rpc.allow-unprotected-txs", "--dev.gaslimit", "200000000"]
