import assert from "node:assert/strict";
import { decideSheetSnap } from "./sheet-drag.snap";

const base = {
  expanded: false,
  dismissMode: false,
  dy: 0,
  ty: 400,
  midTy: 400,
  vh: 900,
};

assert.equal(decideSheetSnap({ ...base, dy: -50, ty: 200 }), "expanded");
assert.equal(decideSheetSnap({ ...base, ty: 100 }), "expanded");
assert.equal(decideSheetSnap({ ...base, expanded: true, dy: 50, ty: 300 }), "mid");
assert.equal(
  decideSheetSnap({ ...base, dismissMode: true, dy: 200, ty: 600 }),
  "dismiss",
);
assert.equal(
  decideSheetSnap({ ...base, dismissMode: true, dy: 40, ty: 420 }),
  "mid",
);

console.log("sheet-drag.check: ok");
