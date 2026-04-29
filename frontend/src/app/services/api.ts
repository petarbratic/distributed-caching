import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';

@Injectable({
  providedIn: 'root',
})
export class Api {
  
  constructor(private http: HttpClient) {}

  gatewayURL = `http://localhost:8080`

  getOne(id: number) {
    return this.http.get(this.gatewayURL + `/api/backend/${id}`);
  }

}
