export async function openPopupWindow(route: string) {
  const popupWidth = 400;
  const popupHeight = 600;

  const currentWindow = await chrome.windows.getCurrent();
  if (!currentWindow) {
    console.error("No current window found.");
    return;
  }

  const left = Math.round(currentWindow.left + (currentWindow.width - popupWidth) / 2);
  const top = Math.round(currentWindow.top + (currentWindow.height - popupHeight) / 2);

  await chrome.windows.create({
    url: chrome.runtime.getURL(`popup.html#${route}`),
    type: "popup",
    width: popupWidth,
    height: popupHeight,
    left,
    top,
  });
}
