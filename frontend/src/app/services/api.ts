import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';

import { CacheConfig } from '../models/cache-config';

@Injectable({
  providedIn: 'root',
})
export class Api {

  gatewayURL = 'http://localhost:8080';

  constructor(private http: HttpClient) {}

  getOne(id: number) {
    return this.http.get(
      this.gatewayURL + `/api/backend/${id}`
    );
  }

  getCacheConfig() {
    return this.http.get<CacheConfig>(
      this.gatewayURL + '/api/cache-config'
    );
  }

  updateCacheConfig(config: CacheConfig) {
    return this.http.put<CacheConfig>(
      this.gatewayURL + '/api/cache-config',
      config
    );
  }
}