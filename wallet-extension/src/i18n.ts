import i18n from "i18next";
import { initReactI18next } from "react-i18next";

import translationEn from "../public/locales/en/translation.json";

const RESOURCES = {
  en: { translation: translationEn },
};

export const defaultNS = "translation";

i18n.use(initReactI18next).init({
  resources: RESOURCES,
  defaultNS,
  fallbackLng: "en",
});

export { i18n };
