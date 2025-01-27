<h1 align="center">docs_ai_backend</h1>

<br />

<p align="center">
  The backend service for the AI chatbot deployed at <a href="https://docs.nil.foundation">docs.nil.foundation</a>.
</p>

## Table of contents

* [Overview](#overview)
* [Installation](#installation)
* [Usage](#usage)
* [License](#license)

## Overview

This project contains the backend service for the AI 'helper' chatbot deployed at the =nil; documentation portal.

The service itself is a Next.js application that exposes two API routes:

* `/api/chat/` is the default endpoint for sending messages to users, processing the response and returning a text stream with said response.
* `/api/services/db` is the endpoint for regenerating the database hosting the vector embeddings of the =nil; documentation. This endpoint should ideally be called every several weeks.

## Installation

Clone the repository:

```bash
git clone https://github.com/NilFoundation/nil.git
cd ./nil/docs_ai_backend
```
Install dependencies:

```bash
npm install
```

## Usage

First, install and deploy [a libSQL database](https://github.com/tursodatabase/libsql/releases). The database has to be available at `http://HOST:PORT`.

Then, acquire a pair of Google reCAPTCHA keys protecting the domain that will send requests to the AI service.

Create an `.env` file with the following structure:

```
OPENAI_API_KEY=
DB_URL=
RECAPTCHA_CLIENT_KEY=
RECAPTCHA_SECRET_KEY=
```

, where `OPENAI_API_KEY` is the API key for the OpenAI API, `DB_URL` is the URL to the newly created libSQL database, and `RECAPTCHA_CLIENT_KEY` and `RECAPTCHA_SECRET_KEY` are the previously acquired reCAPTCHA keys.


To run the service in dev mode at port `7000`:

```bash
npm run dev
```

To create a production build:

```bash
npm run build
```

To start a previously created production build at port `8092`:

```bash
npm run start
```

To populate the DB with embeddings:

```
curl -X POST http://AI_HOST:8092/api/services/db -d ''
```

To query the chatbot:

```
curl -X POST http://AI_HOST:8092/api/chat -d '{"messages": MESSAGES, "token": CAPTCHA_TOKEN}'
```

## License

[MIT](./LICENSE)