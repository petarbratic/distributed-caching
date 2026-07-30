export interface CacheConfig {
  l1MaxEntries: number;
  l2MaxEntries: number;
  l1TTLSeconds: number;
  l2TTLSeconds: number;

  semaphoreSize: number;
  concurrentDelayMs: number;
  baseLatencyMs: number;

  singleFlightEnabled: boolean;
  distributedLockEnabled: boolean;
  probabilisticEarlyRefreshEnabled: boolean;
  earlyRefreshBeta: number;
  backendTimeoutMs: number;

  adaptiveTTLEnabled: boolean;
  adaptiveTTLMinFactor: number;
  adaptiveTTLMaxFactor: number;
  adaptiveTTLLatencyThresholdMs: number;
  adaptiveTTLConcurrencyThreshold: number;
  adaptiveTTLAdjustmentIntervalMs: number;
}

export interface GatewayConfig {
  l1MaxEntries: number;
  l2MaxEntries: number;
  l1TTLSeconds: number;
  l2TTLSeconds: number;
  singleFlightEnabled: boolean;
  distributedLockEnabled: boolean;
  probabilisticEarlyRefreshEnabled: boolean;
  earlyRefreshBeta: number;
  backendTimeoutMs: number;
  adaptiveTTLEnabled: boolean;
  adaptiveTTLMinFactor: number;
  adaptiveTTLMaxFactor: number;
  adaptiveTTLLatencyThresholdMs: number;
  adaptiveTTLConcurrencyThreshold: number;
  adaptiveTTLAdjustmentIntervalMs: number;
}

export interface BackendConfig {
  semaphoreSize: number;
  concurrentDelayMs: number;
  baseLatencyMs: number;
}