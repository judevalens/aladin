// Squarified treemap (Bruls, Huizing & van Wijk) — packs weighted items into a rectangle,
// keeping tile aspect ratios close to square so small items become readable rectangles
// rather than slivers. Used nested: sectors into the map, then tiles into each sector.

export interface Rect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface Weighted<T> {
  value: number;
  data: T;
}

export interface Placed<T> extends Rect {
  data: T;
}

function worst(areas: number[], side: number): number {
  if (areas.length === 0) return Infinity;
  const sum = areas.reduce((s, a) => s + a, 0);
  const max = Math.max(...areas);
  const min = Math.min(...areas);
  const s2 = sum * sum;
  const side2 = side * side;
  return Math.max((side2 * max) / s2, s2 / (side2 * min));
}

/** Squarify `items` into `rect`. Areas are scaled to fill the rect exactly. */
export function squarify<T>(items: Weighted<T>[], rect: Rect): Placed<T>[] {
  const total = items.reduce((s, it) => s + it.value, 0);
  if (total <= 0 || rect.w <= 0 || rect.h <= 0) return [];
  const scale = (rect.w * rect.h) / total;
  const nodes = items
    .map((it) => ({ area: it.value * scale, data: it.data }))
    .sort((a, b) => b.area - a.area);

  const out: Placed<T>[] = [];
  const free: Rect = { ...rect };
  let row: { area: number; data: T }[] = [];

  const placeRow = () => {
    const sum = row.reduce((s, n) => s + n.area, 0);
    if (free.w >= free.h) {
      // A column on the left, tiles stacked vertically.
      const colW = sum / free.h;
      let y = free.y;
      for (const n of row) {
        const tileH = n.area / colW;
        out.push({ data: n.data, x: free.x, y, w: colW, h: tileH });
        y += tileH;
      }
      free.x += colW;
      free.w -= colW;
    } else {
      // A row on the top, tiles side by side.
      const rowH = sum / free.w;
      let x = free.x;
      for (const n of row) {
        const tileW = n.area / rowH;
        out.push({ data: n.data, x, y: free.y, w: tileW, h: rowH });
        x += tileW;
      }
      free.y += rowH;
      free.h -= rowH;
    }
    row = [];
  };

  for (const node of nodes) {
    const side = Math.min(free.w, free.h);
    const current = row.map((n) => n.area);
    if (row.length === 0 || worst(current, side) >= worst([...current, node.area], side)) {
      row.push(node);
    } else {
      placeRow();
      row.push(node);
    }
  }
  if (row.length) placeRow();
  return out;
}
