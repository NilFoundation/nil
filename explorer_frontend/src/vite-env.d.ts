/// <reference types="vite/client" />

declare module "*.sol" {
  const content: string;
  export default content;
}
