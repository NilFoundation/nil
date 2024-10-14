import { createDomain } from "effector";

export const domain = createDomain("sandbox");

export const changeAmount = domain.createEvent<string>();
export const changeAddress = domain.createEvent<string>();

export const $amount = domain.createStore<string>("1");
export const $address = domain.createStore<string>(
  "-1:2eae104ee0016a134090084079894a29346876fe660563fade49d307a6c691dc",
);

export const fetchWalletStateFx = domain.createEffect(async (_publicKey: string) => {
  return {};
});

export const deploy = domain.createEvent();

export const increaseMoney = domain.createEvent();

export const sendMoney = domain.createEvent();
