<h1 align="center">docs.nil.foundation</h1>

<br />

<p align="center">
  The technical documentation for =nil; implemented as a Docusaurus project.
</p>

## Table of contents

* [Overview](#overview)
* [Installation](#installation)
* [Usage](#usage)
* [Tests](#tests)
* [License](#license)

## Overview

This project contains the Docusaurus instance that renders the technical documentation for =nil;.

Docusaurus supports MDX, which is a syntax/file format that combines Markdown and JSX. This means that Markdown rendering, React components and even 'raw' JavaScript can be freely used in the same `.mdx` file. When Docusaurus is run, it renders the contents of `.mdx` files as static HTML pages ready for deployment.

## Installation

Clone the repository:

```bash
git clone https://github.com/NilFoundation/nil.git
cd ./nil/docs
```
Install dependencies:

```bash
npm install
```

## Usage

To launch the Docusaurus instance locally on port `3000`:

```bash
npm run start
```

## Tests

### Explanation

The =nil; documentation comes with an extensive suite of tests located in `./tests`. 

The tests repeat the structure of all major tutorials in the documentation, and tutorials themselves display the code used in the tests. If a test changes, so do the code snippets in the corresponding tutorial.

To achieve this effect, the docs use [`nil-remark-code-snippets`](https://github.com/khannanov-nil/remark-code-snippets), a fork of the original `remark-code-snippets` plugin. Inside a test, code blocks to be displayed in tutorials are placed between comment blocks that typically read `//start...` and `//end...`. Tutorials refer to these comments when opening a new code snippet using the three backticks (```)notation:

```
file=path/to/test start=START_COMMENT end=END_COMMENT
```

, where `file` is the path to the file with a test, `start` is the starting comment preceding the code snippet that is supposed to be displayed and `end` is the comment following the required code block.

### Running tests

Before running tests, launch `nild`, `faucet` and `cometa`. Then:

```bash
npm run test
```

To run an individual test:

```bash
npm run test path/to/test
```

## License

[MIT](./LICENCE)
