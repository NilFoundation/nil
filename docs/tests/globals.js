export const RPC_GLOBAL = process.env.USE_NILD
  ? "http://127.0.0.1:8529"
  : "https://api.devnet.nil.foundation/api/nil_user/TEK83KSDZH58AIK9PCYSNU4G86DU55I9/";
export const COMETA_GLOBAL = process.env.USE_NILD
  ? "http://127.0.0.1:8529"
  : "https://api.devnet.nil.foundation/api/nil_user/TEK83KSDZH58AIK9PCYSNU4G86DU55I9/";
export const FAUCET_GLOBAL = process.env.USE_NILD
  ? "http://127.0.0.1:8527"
  : "https://api.devnet.nil.foundation/api/nil_user/TEK83KSDZH58AIK9PCYSNU4G86DU55I9/";
export const NIL_GLOBAL = "nil";
export const NODE_MODULES = "--include-path ../node_modules/ --base-path ../ ";
