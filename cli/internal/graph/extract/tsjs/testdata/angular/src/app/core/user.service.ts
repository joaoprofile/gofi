import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';

export interface User {
  id: string;
  name: string;
}

@Injectable({ providedIn: 'root' })
export class UserService {
  constructor(private http: HttpClient) {}

  list(): User[] {
    return [];
  }

  private endpoint(): string {
    return '/api/users';
  }
}
