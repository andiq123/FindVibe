/** ponytail: window math for vault chunking — fails if slice/cap drifts. */
describe("song-list chunk window", () => {
  const slice = (all: string[], chunk: number, visible: number) => {
    if (chunk <= 0) return all;
    const cap = visible > 0 ? Math.min(visible, all.length) : Math.min(chunk, all.length);
    return all.slice(0, cap);
  };

  it("shows first page then grows", () => {
    const all = Array.from({ length: 50 }, (_, i) => `s${i}`);
    expect(slice(all, 24, 0).length).toBe(24);
    expect(slice(all, 24, 24).length).toBe(24);
    expect(slice(all, 24, 48).length).toBe(48);
    expect(slice(all, 24, 100).length).toBe(50);
  });

  it("clamps when list shrinks", () => {
    const all = ["a", "b", "c"];
    expect(slice(all, 24, 40).length).toBe(3);
  });
});
