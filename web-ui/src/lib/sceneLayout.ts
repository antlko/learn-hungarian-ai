/**
 * Where things go in a `scene` picture, per place drawing.
 *
 * Everything is in the place drawings' own 160×120 box. A place is drawn once
 * (backend/internal/pictures/svg/<id>.svg) and these anchors say where an
 * object rests on/in it and where it comes from or goes to. They are tied to
 * the drawings: change a drawing, check its row.
 */
export const BOX_W = 160
export const BOX_H = 120
/** Side of the object's tile, in box units. */
export const TILE = 36

export interface Slot {
  x: number
  y: number
}
export interface Rect {
  x: number
  y: number
  w: number
  h: number
}
export interface RelLayout {
  /** Top-left of the tile where the object rests at the place. */
  rest: Slot
  /** Top-left of the tile it comes from (`into`). */
  into: Slot
  /** Top-left of the tile it goes to (`out`). */
  out: Slot
  /**
   * For `in`: the front face of the container. Drawn OVER the object, with an
   * opaque background, so the object reads as inside, not in front of it.
   */
  front?: Rect
}
export type PlaceLayout = { in?: RelLayout; on?: RelLayout }

const TOP_L: Slot = { x: 4, y: 4 }
const TOP_R: Slot = { x: 120, y: 4 }

export const SCENE_LAYOUT: Record<string, PlaceLayout> = {
  box: {
    in: { rest: { x: 62, y: 38 }, into: TOP_L, out: TOP_R, front: { x: 26, y: 60, w: 108, h: 56 } },
    on: { rest: { x: 62, y: 10 }, into: { x: 4, y: 10 }, out: { x: 120, y: 10 } },
  },
  wardrobe: {
    in: { rest: { x: 62, y: 62 }, into: TOP_L, out: TOP_R },
    on: { rest: { x: 62, y: 8 }, into: { x: 4, y: 8 }, out: { x: 120, y: 8 } },
  },
  table: { on: { rest: { x: 62, y: 16 }, into: { x: 4, y: 6 }, out: { x: 120, y: 6 } } },
  shelf: { on: { rest: { x: 34, y: 40 }, into: { x: 122, y: 0 }, out: { x: 122, y: 0 } } },
  chair: { on: { rest: { x: 64, y: 34 }, into: { x: 122, y: 10 }, out: { x: 122, y: 10 } } },
  bag: { in: { rest: { x: 62, y: 30 }, into: TOP_L, out: TOP_R, front: { x: 28, y: 50, w: 104, h: 66 } } },
  basket: { in: { rest: { x: 62, y: 28 }, into: TOP_L, out: TOP_R, front: { x: 20, y: 50, w: 120, h: 64 } } },
  house: { in: { rest: { x: 66, y: 76 }, into: { x: 122, y: 76 }, out: { x: 122, y: 76 } } },
  drawer: { in: { rest: { x: 62, y: 34 }, into: { x: 4, y: 2 }, out: { x: 120, y: 2 }, front: { x: 20, y: 54, w: 120, h: 58 } } },
  cup: { in: { rest: { x: 62, y: 14 }, into: TOP_L, out: TOP_L, front: { x: 46, y: 42, w: 68, h: 68 } } },
}

const FALLBACK: RelLayout = { rest: { x: 62, y: 42 }, into: TOP_L, out: TOP_R }

export function layoutFor(place: string, relation: 'in' | 'on'): RelLayout {
  return SCENE_LAYOUT[place]?.[relation] ?? FALLBACK
}

export interface Geometry {
  start: Slot
  end: Slot
  /** Null for a static scene. */
  arrow: { d: string; head: string; length: number } | null
  front?: Rect
}

/**
 * The arrow runs from where the object IS to where it will be: the occupied end
 * is trimmed to clear its tile, the empty end stops just short of the slot's
 * centre, so the tail is long enough to read as a line, not a dot.
 */
export function sceneGeometry(
  place: string,
  relation: 'in' | 'on',
  motion: 'static' | 'into' | 'out',
): Geometry {
  const l = layoutFor(place, relation)
  const front = relation === 'in' ? l.front : undefined
  if (motion === 'static') return { start: l.rest, end: l.rest, arrow: null, front }
  const start = motion === 'into' ? l.into : l.rest
  const end = motion === 'into' ? l.rest : l.out
  const c = TILE / 2
  const s = { x: start.x + c, y: start.y + c }
  const e = { x: end.x + c, y: end.y + c }
  const dx = e.x - s.x
  const dy = e.y - s.y
  const len = Math.hypot(dx, dy) || 1
  const ux = dx / len
  const uy = dy / len
  const trim = c / Math.max(Math.abs(ux), Math.abs(uy)) + 2
  const a = { x: s.x + ux * trim, y: s.y + uy * trim }
  const b = { x: e.x - ux * 3, y: e.y - uy * 3 }
  const ctrl = { x: (a.x + b.x) / 2 - uy * 12, y: (a.y + b.y) / 2 + ux * 12 }
  const tx = b.x - ctrl.x
  const ty = b.y - ctrl.y
  const tl = Math.hypot(tx, ty) || 1
  const hx = tx / tl
  const hy = ty / tl
  const head = [
    [b.x + hx * 3, b.y + hy * 3],
    [b.x - hx * 9 - hy * 6, b.y - hy * 9 + hx * 6],
    [b.x - hx * 9 + hy * 6, b.y - hy * 9 - hx * 6],
  ]
    .map((p) => p.map((n) => n.toFixed(1)).join(','))
    .join(' ')
  return {
    start,
    end,
    arrow: {
      d: `M${a.x.toFixed(1)},${a.y.toFixed(1)} Q${ctrl.x.toFixed(1)},${ctrl.y.toFixed(1)} ${(b.x - hx * 6).toFixed(1)},${(b.y - hy * 6).toFixed(1)}`,
      head,
      length: Math.hypot(b.x - a.x, b.y - a.y),
    },
    front,
  }
}
