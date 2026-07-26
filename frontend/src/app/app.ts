import { Component, OnInit, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { Api } from './services/api';
import { CacheConfig, BackendConfig, GatewayConfig } from './models/cache-config';
import { catchError, forkJoin, map, of } from 'rxjs';

@Component({
  selector: 'app-root',
  imports: [FormsModule],
  templateUrl: './app.html',
  styleUrl: './app.css'
})
export class App implements OnInit {
  protected readonly title = signal('frontend');

  id: number = 0;

  cacheConfig: CacheConfig = {
    l1MaxEntries: 3,
    l2MaxEntries: 10,
    l1TTLSeconds: 2,
    l2TTLSeconds: 4,

    semaphoreSize: 3,
    concurrentDelayMs: 200,
    baseLatencyMs: 2000,

    singleFlightEnabled: false,
    distributedLockEnabled: false,
    backendTimeoutMs: 5000,
  };

  loading = false;
  successMessage = '';
  errorMessage = '';

  constructor(private api: Api) { }

  ngOnInit(): void {
    this.loadCacheConfig();
  }

  load(): void {
    this.api.getOne(this.id).subscribe({
      next: res => {
        console.log(res);
      },
      error: err => {
        console.error('Backend request error:', err);
      }
    });
  }

  loadCacheConfig(): void {
    this.loading = true;
    this.errorMessage = '';

    forkJoin({
      gateway: this.api.getGatewayConfig(),
      backend: this.api.getBackendConfig(),
    }).subscribe({
      next: ({ gateway, backend }) => {
        this.cacheConfig = {
          ...gateway,
          ...backend,
        };

        this.loading = false;
      },
      error: err => {
        console.error('Configuration GET error:', err);

        this.errorMessage =
          'Unable to load system configuration.';

        this.loading = false;
      }
    });
  }

  saveCacheConfig(): void {
    this.successMessage = '';
    this.errorMessage = '';

    if (this.cacheConfig.l1MaxEntries <= 0) {
      this.errorMessage = 'L1 size must be greater than 0.';
      return;
    }

    if (this.cacheConfig.l2MaxEntries <= 0) {
      this.errorMessage = 'L2 size must be greater than 0.';
      return;
    }

    if (
      this.cacheConfig.l1MaxEntries >
      this.cacheConfig.l2MaxEntries
    ) {
      this.errorMessage =
        'L1 size cannot be greater than L2 size.';
      return;
    }

    if (this.cacheConfig.l1TTLSeconds <= 0) {
      this.errorMessage = 'L1 TTL must be greater than 0.';
      return;
    }

    if (this.cacheConfig.l2TTLSeconds <= 0) {
      this.errorMessage = 'L2 TTL must be greater than 0.';
      return;
    }

    if (this.cacheConfig.l1TTLSeconds > this.cacheConfig.l2TTLSeconds) {
      this.errorMessage = 'L1 TTL cannot be greater than L2 TTL.';
      return;
    }

    if (this.cacheConfig.semaphoreSize <= 0) {
      this.errorMessage = 'Semaphore size must be greater than 0.';
      return;
    }

    this.loading = true;

    const gatewayConfig: GatewayConfig = {
      l1MaxEntries: this.cacheConfig.l1MaxEntries,
      l2MaxEntries: this.cacheConfig.l2MaxEntries,
      l1TTLSeconds: this.cacheConfig.l1TTLSeconds,
      l2TTLSeconds: this.cacheConfig.l2TTLSeconds,
      singleFlightEnabled:
        this.cacheConfig.singleFlightEnabled,
      distributedLockEnabled:
        this.cacheConfig.distributedLockEnabled,
      backendTimeoutMs:
        this.cacheConfig.backendTimeoutMs,
    };

    const backendConfig: BackendConfig = {
      semaphoreSize: this.cacheConfig.semaphoreSize,
      concurrentDelayMs:
        this.cacheConfig.concurrentDelayMs,
      baseLatencyMs: this.cacheConfig.baseLatencyMs,
    };

    const gatewayRequest =
      this.api.updateGatewayConfig(gatewayConfig).pipe(
        map(config => ({
          success: true as const,
          config,
        })),
        catchError(error => of({
          success: false as const,
          error,
        }))
      );

    const backendRequest =
      this.api.updateBackendConfig(backendConfig).pipe(
        map(config => ({
          success: true as const,
          config,
        })),
        catchError(error => of({
          success: false as const,
          error,
        }))
      );

    forkJoin({
      gateway: gatewayRequest,
      backend: backendRequest,
    }).subscribe({
      next: result => {
        this.loading = false;

        if (result.gateway.success) {
          this.cacheConfig = {
            ...this.cacheConfig,
            ...result.gateway.config,
          };
        }

        if (result.backend.success) {
          this.cacheConfig = {
            ...this.cacheConfig,
            ...result.backend.config,
          };
        }

        if (
          result.gateway.success &&
          result.backend.success
        ) {
          this.successMessage =
            'Configuration successfully updated. Caches have been cleared.';
          return;
        }

        if (
          result.gateway.success &&
          !result.backend.success
        ) {
          console.error(
            'Backend config PUT error:',
            result.backend.error
          );

          this.errorMessage =
            'Gateway configuration was updated, but backend configuration failed.';
          return;
        }

        if (
          !result.gateway.success &&
          result.backend.success
        ) {
          console.error(
            'Gateway config PUT error:',
            result.gateway.error
          );

          this.errorMessage =
            'Backend configuration was updated, but gateway configuration failed.';
          return;
        }

        if ('error' in result.gateway) {
          console.error(
            'Gateway config PUT error:',
            result.gateway.error
          );
        }

        if ('error' in result.backend) {
          console.error(
            'Backend config PUT error:',
            result.backend.error
          );
        }

        this.errorMessage =
          'Unable to update gateway and backend configuration.';
      }
    });
  }
}