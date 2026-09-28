import { readSearchStream } from "./search-stream";

describe("search stream framing", () => {
  it("preserves UTF-8 and JSON split across arbitrary chunks, including final lines", async () => {
    const bytes = new TextEncoder().encode('{"type":"song","song":{"title":"Bôa — Привет"}}\n{"type":"done"}');
    const response = new Response(new ReadableStream({ start(controller) {
      for (const byte of bytes) controller.enqueue(new Uint8Array([byte]));
      controller.close();
    } }));
    const events = [];
    for await (const event of readSearchStream(response)) events.push(event);
    expect(events[0].song?.title).toBe("Bôa — Привет");
    expect(events[1].type).toBe("done");
  });
  it("rejects unsuccessful responses", async () => {
    const read = async () => { for await (const event of readSearchStream(new Response("", { status: 503 }))) void event; };
    await expectAsync(read()).toBeRejectedWithError("Search is unavailable");
  });
  it("cancels the reader when the consumer stops early", async () => {
    const cancel = jasmine.createSpy("cancel");
    const response = new Response(new ReadableStream({ start(controller) {
      controller.enqueue(new TextEncoder().encode('{"type":"done"}\n'));
    }, cancel }));
    for await (const event of readSearchStream(response)) { expect(event.type).toBe("done"); break; }
    expect(cancel).toHaveBeenCalled();
  });
});
