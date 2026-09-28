import { Injectable, inject } from "@angular/core";
import { environment } from "../../../../environments/environment";
import { HttpClient } from "@angular/common/http";
import { Song } from "../../../core/models/song.model";
import { Observable } from "rxjs";
import { Reorder } from "../../../core/models/reorder.model";
const BASE_API_URL = environment.API_URL;
@Injectable({
  providedIn: "root",
})
export class LibraryApiService {
  private readonly httpClient = inject(HttpClient);
  getFavoritesSong(userId: string): Observable<Song[]> {
    return this.httpClient.get<Song[]>(`${BASE_API_URL}/favorites/${userId}`);
  }
  reorderSongs(reorders: Reorder[]): Observable<unknown> {
    return this.httpClient.put(`${BASE_API_URL}/favorites`, reorders);
  }
  addToFavorites(song: Song, userId: string): Observable<unknown> {
    return this.httpClient.post(`${BASE_API_URL}/favorites/${userId}`, song);
  }
  removeFromFavorites(songId: string): Observable<unknown> {
    return this.httpClient.delete(`${BASE_API_URL}/favorites/${songId}`);
  }
  updateFavoriteImage(songId: string, image: string): Observable<unknown> {
    return this.httpClient.patch(`${BASE_API_URL}/favorites/${songId}/image`, {
      image,
    });
  }
  updateFavoriteLyrics(songId: string, lyrics: string): Observable<unknown> {
    return this.httpClient.patch(`${BASE_API_URL}/favorites/${songId}/lyrics`, {
      lyrics,
    });
  }
}
