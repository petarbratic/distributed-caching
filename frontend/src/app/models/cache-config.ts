export interface CacheConfig {
  l1MaxEntries: number;
  l2MaxEntries: number;

  l1TTLSeconds: number;
  l2TTLSeconds: number;

  semaphoreSize: number;
}