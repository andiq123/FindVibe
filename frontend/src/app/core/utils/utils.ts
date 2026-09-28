export const convertTime = (time: number) => {
  const minutes = Math.floor(time / 60);
  const seconds = Math.floor(time % 60);
  if (isNaN(minutes) || isNaN(seconds)) {
    return "0:00";
  }
  return `${minutes}:${seconds < 10 ? "0" : ""}${seconds}`;
};
export const bytesToGB = (bytes: number): number => {
  return parseFloat((bytes / 1024 ** 3).toFixed(2));
};
export function shuffleArray<T>(array: T[]): T[] {
  const shuffled = [...array];
  for (let i = shuffled.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [shuffled[i], shuffled[j]] = [shuffled[j], shuffled[i]];
  }
  return shuffled;
}
export function upgradeToHttps(url: string): string {
  return url.replace(/^http:\/\//i, "https://");
}
