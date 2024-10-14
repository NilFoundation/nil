import { metaMask } from "../../connectors/metamask";
import { $token, domain } from "./models/model";

const createEvent = domain.createEvent.bind(domain);
const createEffect = domain.createEffect.bind(domain);

export const connected = createEvent();
export const authorized = createEvent();
export const notAuthorized = createEvent();
export const disconnect = createEvent();

export const connect = createEffect(async () => {
  await metaMask.activate();
});

export const login = createEffect<string, string>(async () => {
  throw new Error("Not implemented");
});

$token.on(login.doneData, (_, token) => token);

export const accountEvent = createEvent<string>();
