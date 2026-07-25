import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';

import { BackendConfig, GatewayConfig } from '../models/cache-config';

@Injectable({
  providedIn: 'root',
})
export class Api {

  gatewayURL = 'http://localhost:8080';
  backendURL = 'http://localhost:8081';

  constructor(private http: HttpClient) { }

  getOne(id: number) {
    return this.http.get(
      this.gatewayURL + `/api/backend/${id}`
    );
  }

  getGatewayConfig() {
    return this.http.get<GatewayConfig>(
      this.gatewayURL + '/api/cache-config'
    );
  }

  updateGatewayConfig(config: GatewayConfig) {
    return this.http.put<GatewayConfig>(
      this.gatewayURL + '/api/cache-config',
      config
    );
  }

  getBackendConfig() {
    return this.http.get<BackendConfig>(
      this.backendURL + '/config'
    );
  }

  updateBackendConfig(config: BackendConfig) {
    return this.http.put<BackendConfig>(
      this.backendURL + '/config',
      config
    );
  }
}