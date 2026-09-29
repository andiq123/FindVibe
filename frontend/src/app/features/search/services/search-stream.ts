import { PaginationInfo, Song } from "../../../core/models/song.model";

export interface SearchEvent {
  type: "meta" | "song" | "done" | "error";
  song?: Song;
  songs?: Song[];
  key?: string;
  pagination?: PaginationInfo;
  error?: string;
}

/** NDJSON frames can cross both network chunks and UTF-8 character boundaries. */
export async function* readSearchStream(response: Response): AsyncGenerator<SearchEvent> {
  if (!response.ok || !response.body) throw new Error("Search is unavailable");
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  try {
    while (true) {
      const { value, done } = await reader.read();
      buffer += decoder.decode(value, { stream: !done });
      const lines = buffer.split("\n");
      buffer = lines.pop() ?? "";
      if (done && buffer.trim()) lines.push(buffer);
      for (const line of lines) {
        if (line.trim()) yield JSON.parse(line) as SearchEvent;
      }
      if (done) return;
    }
  } finally {
    await reader.cancel().catch(() => undefined);
    reader.releaseLock();
  }
}
