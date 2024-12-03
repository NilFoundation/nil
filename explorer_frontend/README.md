# =nil; explorer frontend

This is the frontend for the explorer. It is a React app that uses the [Styletron-react](https://styletron.org/react) library for styling. State management is done using the [Effectorjs](https://effector.dev) library. The app is built using [Vite](https://vitejs.dev).

## Development

Initially you need to install npm packages in the root directory of the project:

```bash
npm ci 
```

Then you need to fill the required config varibales. Frontend app uses runtime config `runtime-config.toml`, stored in /public folder.
You can create `runtime-config.local.toml` file in the /public directory to override the default values.
Generally, only `API_URL` is required to be set. You can copy the content of `runtime-config.toml` to `runtime-config.local.toml` and set the `API_URL` to the correct value.

To start the development server of the app, run:

```bash
npm run dev
```

This will start the development server on port 5173.
You can specify a different port by setting the `PORT` environment variable.

Install biome extension for VS Code to get the best development experience. You can find it [here](https://marketplace.visualstudio.com/items?itemName=biomejs.biome). It will enable code formatting on save and paste.

## Production

To build the app for production, run:

```bash
npm run build
```

This will create a `dist` directory with the built app.
