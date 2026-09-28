import { Injectable, signal, inject } from '@angular/core';
import { StorageService } from '../../../core/services/storage.service';
import { User } from '../../../core/models/user.model';
import { HttpClient } from '@angular/common/http';
import { environment } from "../../../../environments/environment";
import { Subject, tap } from 'rxjs';
import { OfflineStorageService } from './offline-storage.service';
const USER_STORAGE_KEY = 'user';
@Injectable({
  providedIn: 'root',
})
export class UserService {
  private readonly storageService = inject(StorageService);
  private readonly httpClient = inject(HttpClient);
  private readonly offlineStorageService = inject(OfflineStorageService);
  private readonly _user = signal<User | null>(null);
  readonly user = this._user.asReadonly();
  readonly userLoggedIn = new Subject<void>();
  constructor() {
    this.loadUserIdFromStorage();
  }
  private loadUserIdFromStorage(): string | null {
    try {
      const user = this.storageService.getItem<User>(USER_STORAGE_KEY);
      if (user?.id) {
        this._user.set(user);
        return user.id;
      }
    } catch (error) {
      console.error('[UserService] Failed to parse user from storage:', error);
      this.storageService.removeItem(USER_STORAGE_KEY);
    }
    return null;
  }
  registerUser(userName: string) {
    const normalizedUserName = userName.toLowerCase();
    const url = `${environment.API_URL}/${normalizedUserName}`;
    return this.httpClient.get<User>(url).pipe(
      tap({
        next: (user: User) => {
          this._user.set(user);
          this.storageService.setItem(USER_STORAGE_KEY, user);
          this.userLoggedIn.next();
        },
      })
    );
  }
  resetUser(): void {
    this._user.set(null);
    this.storageService.removeItem(USER_STORAGE_KEY);
    this.storageService.removeItem('library');
    this.offlineStorageService.removeCache().catch(err => 
      console.error('[UserService] Failed to clear offline storage:', err)
    );
  }
}
