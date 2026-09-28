import { ActivatedRouteSnapshot } from "@angular/router";
import { CustomReuseStrategy } from "./custom-reuse-strategy";

const route = (path: string, params = {}) => ({ routeConfig: { path }, params }) as ActivatedRouteSnapshot;

describe("route retention", () => {
  const strategy = new CustomReuseStrategy();
  it("does not retain a live screen per search query", () => {
    expect(strategy.shouldDetach(route("songs"))).toBeFalse();
    expect(strategy.shouldDetach(route("songs/:query", { query: "Daft Punk" }))).toBeFalse();
    expect(strategy.shouldDetach(route("songs/:query", { query: "Adele" }))).toBeFalse();
  });
  it("preserves the finite library and recent tabs", () => {
    expect(strategy.shouldDetach(route("library"))).toBeTrue();
    expect(strategy.shouldDetach(route("recent"))).toBeTrue();
  });
  it("reuses the active search screen when only its query changes", () => {
    expect(strategy.shouldReuseRoute(route("songs"), route("songs/:query"))).toBeTrue();
  });
});
