<h1 align="center">explorer_backend</h1>

<br />

<p align="center">
  The backend for the =nil; block explorer and the Playground.
</p>

<br />

## Table of contents

* [Overview](#overview)
* [Installation](#installation)
* [Development](#development)

## Overview

This project contains the backend component for [the =nil; block explorer](https://explore.nil.foundation/). It is tRPC API that provides several endpoints which are consumed by `explorer_frontend`. 

## Installation

Clone the repository:

```bash
git clone https://github.com/NilFoundation/nil.git
cd ./nil/explorer_backend
```
Install dependencies:

```bash
npm install
```

## Development

An existing instance of [ClickHouse](https://clickhouse.com/) is needed to run `explorer_backend`.

After deploying the instance, create an `.env` file and provide credentials to the instance:

```
DB_URL:
DB_USER:
DB_PATHNAME:
DB_PASSWORD:
DB_NAME:
```

NB: the project only supports `http://` to connect to the provided ClickHouse instance.

Additionally, set the RPC URL:

```
RPC_URL:
```

Launch the project with:

```bash
npm run start
```