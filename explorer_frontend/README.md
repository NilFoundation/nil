<h1 align="center">explorer_frontend</h1>

<br />

<p align="center">
  The frontend for the =nil; block explorer and the Playground.
</p>

<br />

## Table of contents

## Overview

This project contains the front-end component for [the =nil; block explorer](https://explore.nil.foundation/). It is a React app that uses the [Styletron-react](https://styletron.org/react) library for styling. State management is done using the [Effectorjs](https://effector.dev) library. The app is built using [Vite](https://vitejs.dev).

## Installation

Clone the repository:

```bash
git clone https://github.com/NilFoundation/nil.git
cd ./nil/explorer_frontend
```
Install dependencies:

```bash
npm install
```

## Development

Fill the required config variables in the `runtime-config.toml` file stored in `./public`. Presently, only `API_URL` is required to be set.

To override the default values, create the `runtime-config.local.toml` file in `./public` and set `API_URL` to the desired value.

To start the development server:

```bash
npm run dev
```

This will start the development server on port `5173`.

A different port can be set by specifying the `PORT` environment variable.

Install [the `biome` extension](https://marketplace.visualstudio.com/items?itemName=biomejs.biome) for VS Code for the smoothest possible development experience. It will enable code formatting on save and paste.

## Production

To build the app for production:

```bash
npm run build
```

This will create a `dist` directory with the built app.

## Testing

Explorer frontend used cypress for end-to-end testing. To run the tests initialize the http server and run:

```bash
export PORT=3000 # cypress expects the app on port 3000 by default
npm run dev # initializes the server in development mode which is suitable for real-time testing
```

or

```bash
npm run build
npm run serve # initializes the server in production mode which is suitable for testing the production build
```

In this mode, it is also necessary to disable API requests batching in the `runtime-config.toml` file by setting `API_REQUESTS_ENABLE_BATCHING` to `false`.

Then, in a separate terminal, run:

```bash
npm run test:e2e
```

This will open the cypress test runner where it should be possible to select the test file to run.

To run tests in headless mode:

```bash
npm run test:e2e:ci
```
