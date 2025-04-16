import type { StyleObject } from "styletron-react";

export const mobileMaxScreenSize = 1024;
export const mediumMaxScreenSize = 1920;

export const getMobileStyles = (styles: StyleObject) => ({
  [`@media screen and (max-width: ${mobileMaxScreenSize}px)`]: {
    ...styles,
  },
});

export const getLargeScreenStyles = (styles: StyleObject) => ({
  [`@media screen and (min-width: ${mediumMaxScreenSize + 1}px)`]: {
    ...styles,
  },
});
