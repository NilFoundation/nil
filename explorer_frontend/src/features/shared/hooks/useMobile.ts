import { useEffect, useState } from "react";
import { mobileMaxScreenSize } from "../../../styleHelpers";

const mql = window.matchMedia(`(max-width: ${mobileMaxScreenSize}px)`);
let currentValue = mql.matches;

const subs: ((isMobile: boolean) => void)[] = [];

if (mql.addEventListener) {
  mql.addEventListener("change", (e: MediaQueryListEvent) => {
    const isMobile = e.matches;
    if (currentValue !== isMobile) {
      currentValue = isMobile;
      subs.forEach((sub) => sub(isMobile));
    }
  });
} else {
  mql.addListener((e: MediaQueryListEvent) => {
    const isMobile = e.matches;
    if (currentValue !== isMobile) {
      currentValue = isMobile;
      subs.forEach((sub) => sub(isMobile));
    }
  });
}

export const useMobile = () => {
  const [isMobile, setMobile] = useState(currentValue);
  useEffect(() => {
    subs.push(setMobile);
    return () => {
      subs.splice(subs.indexOf(setMobile), 1);
    };
  }, []);
  return [isMobile];
};
