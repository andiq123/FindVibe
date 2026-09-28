/** Snap decision after a sheet drag ends (transform / translateY model). */
export function decideSheetSnap(input: {
  expanded: boolean;
  dismissMode: boolean;
  dy: number;
  ty: number;
  midTy: number;
  vh: number;
}): "dismiss" | "mid" | "expanded" {
  const { expanded, dismissMode, dy, ty, midTy, vh } = input;
  if (dismissMode || ty > midTy + 24) {
    return ty > midTy + vh * 0.12 || dy > 110 ? "dismiss" : "mid";
  }
  const expandLine = midTy * 0.55;
  if (dy < -36 || ty <= expandLine) return "expanded";
  if (expanded && dy > 36) return "mid";
  return expanded ? "expanded" : "mid";
}
