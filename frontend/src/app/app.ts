import { Component, OnInit, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { Api } from './services/api';
import { CacheConfig } from './models/cache-config';


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
    l1MaxEntries: 0,
    l2MaxEntries: 0,
    l1TTLSeconds: 0,
    l2TTLSeconds: 0,
    semaphoreSize: 0
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

    this.api.getCacheConfig().subscribe({
      next: config => {
        this.cacheConfig = config;
        this.loading = false;
      },
      error: err => {
        console.error('Cache config GET error:', err);
        this.errorMessage = 'Unable to load cache configuration.';
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

    this.api.updateCacheConfig(this.cacheConfig).subscribe({
      next: config => {
        this.cacheConfig = config;
        this.successMessage =
          'Configuration successfully updated. Caches have been cleared.';
        this.loading = false;
      },
      error: err => {
        console.error('Cache config PUT error:', err);

        this.errorMessage =
          typeof err.error === 'string'
            ? err.error
            : 'Unable to update cache configuration.';

        this.loading = false;
      }
    });
  }
}