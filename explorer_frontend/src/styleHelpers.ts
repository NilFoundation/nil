import type { StyleObject } from "styletron-react";

export const mobileMaxScreenSize = 768;
export const tabletMaxScreenSize = 1024;
export const mediumMaxScreenSize = 1920;

export const getMobileStyles = (styles: StyleObject) => ({
  [`@media screen and (max-width: ${mobileMaxScreenSize}px)`]: {
    ...styles,
  },
});

export const getTabletStyles = (styles: StyleObject) => ({
  [`@media screen and (min-width: ${mobileMaxScreenSize + 1}px) and (max-width: ${tabletMaxScreenSize}px)`]:
    {
      ...styles,
    },
});

export const getMediumScreenStyles = (styles: StyleObject) => ({
  [`@media screen and (min-width: ${tabletMaxScreenSize + 1}px) and (max-width: ${mediumMaxScreenSize}px)`]:
    {
      ...styles,
    },
});

export const getLargeScreenStyles = (styles: StyleObject) => ({
  [`@media screen and (min-width: ${mediumMaxScreenSize + 1}px)`]: {
    ...styles,
  },
});
