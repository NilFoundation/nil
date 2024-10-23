import { config } from "../config";
import { LRUCache } from "lru-cache";

const CACHE_ITEM_LIMIT = 1000;

const lru = new LRUCache({
  max: CACHE_ITEM_LIMIT,
});

export enum CacheStatus {
  HIT = "HIT",
  MISS = "MISS",
  EXPIRED = "EXPIRED",
}

const cache = new Map<string, [unknown, number]>();

const promiseMap = new Map<string, Promise<unknown>>();

const INTERVAL_CACHE_CHECKER = config.INTERVAL_CACHE_CHECKER;
const CACHE_DEADLINE = config.CACHE_DEADLINE;

export enum CacheType {
  TIMER = "TIMER",
  LRU = "LRU",
}

export type CacheSettings =
  | {
      type: CacheType.TIMER;
      time: number;
      actuality?: boolean;
    }
  | {
      type: CacheType.LRU;
    };

type CacheResult<T extends {}> =
  | [null, CacheStatus.MISS]
  | [T, CacheStatus.HIT]
  | [T, CacheStatus.EXPIRED];
export const getCache = <T extends {}>(key: string, type: CacheType): CacheResult<T> => {
  switch (type) {
    case CacheType.LRU: {
      const value = lru.get(key);
      if (!value) {
        return [null, CacheStatus.MISS];
      }
      return [value as T, CacheStatus.HIT];
    }
    case CacheType.TIMER: {
      const value = cache.get(key);
      if (!value) {
        return [null, CacheStatus.MISS];
      }

      if (value[1] < Date.now()) {
        return [value[0] as T, CacheStatus.EXPIRED];
      }
      return [value[0] as T, CacheStatus.HIT];
    }
  }
};

export const setCache = <T extends {}>(key: string, value: T, settings: CacheSettings) => {
  switch (settings.type) {
    case CacheType.LRU:
      lru.set(key, value);
      return;
    case CacheType.TIMER:
      cache.set(key, [value, Date.now() + settings.time]);
      return;
  }
};

const setterHandling = async <T extends {}>(
  key: string,
  setter: () => Promise<T>,
  settings: CacheSettings,
): Promise<T> => {
  const promise = promiseMap.get(key) as Promise<T>;
  if (promise) {
    return promise;
  }
  const newPromise = setter();
  promiseMap.set(key, newPromise);
  try {
    const result = await newPromise;
    switch (settings.type) {
      case CacheType.TIMER:
        cache.set(key, [result, Date.now() + settings.time]);
        break;
      case CacheType.LRU:
        lru.set(key, result);
        break;
    }
    return result;
  } finally {
    promiseMap.delete(key);
  }
};

export const getCacheWithSetter = async <T extends {}>(
  key: string,
  setter: () => Promise<T>,
  settings: CacheSettings,
): Promise<[T, CacheStatus.HIT] | [T, CacheStatus.EXPIRED]> => {
  switch (settings.type) {
    case CacheType.LRU: {
      const lruValue = lru.get(key);
      if (lruValue) {
        return [lruValue as T, CacheStatus.HIT];
      }
      const value = await setterHandling(key, setter, settings);
      return [value, CacheStatus.HIT];
    }
    case CacheType.TIMER: {
      const cacheValue = cache.get(key);
      if (cacheValue) {
        if (cacheValue[1] < Date.now() && !settings.actuality) {
          setterHandling(key, setter, settings);
          return [cacheValue[0] as T, CacheStatus.EXPIRED];
        }
        return [cacheValue[0] as T, CacheStatus.HIT];
      }
      const value = await setterHandling(key, setter, settings);
      return [value, CacheStatus.EXPIRED];
    }
  }
};

let cacheInterval: NodeJS.Timeout;
export const startCacheInterval = () => {
  if (cacheInterval) {
    clearInterval(cacheInterval);
  }
  cacheInterval = setInterval(() => {
    const keys = Array.from(cache.keys());
    for (const key of keys) {
      const value = cache.get(key);
      if (value && value[1] < Date.now() + CACHE_DEADLINE) {
        if (!promiseMap.has(key)) {
          cache.delete(key);
        }
      }
    }
  }, INTERVAL_CACHE_CHECKER);
};
