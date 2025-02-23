export async function saveToStorage<T>(key: string, value: T): Promise<void> {
  try {
    await chrome.storage.local.set({ [key]: value });
    console.log(`Saved ${key} to storage.`);
  } catch (error) {
    console.error(`Failed to save ${key}:`, error);
  }
}

export async function getFromStorage(key: string) {
  return new Promise((resolve) => {
    chrome.storage.local.get([key], (result) => {
      resolve(result[key]);
    });
  });
}

export async function removeFromStorage(key: string) {
  try {
    await chrome.storage.local.remove(key);
    console.log(`Removed ${key} from storage.`);
  } catch (error) {
    console.error(`Failed to remove ${key}:`, error);
  }
}
