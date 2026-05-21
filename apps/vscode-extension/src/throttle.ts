// Trailing-edge throttle: calls within `windowMs` of an in-flight invocation
// collapse into a single call that fires `windowMs` after the most recent
// trigger. Mirrors the typical "I just want to refresh once after a burst
// of events" UX, without needing lodash.
//
// Pure — takes an injectable `now` and timer pair so the unit test can
// drive virtual time.

export interface ThrottleClock {
  now(): number;
  setTimeout(cb: () => void, ms: number): unknown;
  clearTimeout(handle: unknown): void;
}

const realClock: ThrottleClock = {
  now: () => Date.now(),
  setTimeout: (cb, ms) => setTimeout(cb, ms),
  clearTimeout: (h) => clearTimeout(h as ReturnType<typeof setTimeout>),
};

export function throttleTrailing<T extends (...args: any[]) => void>(
  fn: T,
  windowMs: number,
  clock: ThrottleClock = realClock,
): T & { flush: () => void; cancel: () => void } {
  let lastFire = -Infinity;
  let pendingArgs: Parameters<T> | undefined;
  let timer: unknown;

  function schedule(delay: number, args: Parameters<T>) {
    pendingArgs = args;
    if (timer !== undefined) clock.clearTimeout(timer);
    timer = clock.setTimeout(() => {
      timer = undefined;
      const a = pendingArgs!;
      pendingArgs = undefined;
      lastFire = clock.now();
      fn(...a);
    }, delay);
  }

  const wrapped = ((...args: Parameters<T>) => {
    const now = clock.now();
    const since = now - lastFire;
    if (since >= windowMs && timer === undefined) {
      lastFire = now;
      fn(...args);
    } else {
      const delay = Math.max(0, windowMs - since);
      schedule(delay, args);
    }
  }) as T & { flush: () => void; cancel: () => void };

  wrapped.flush = () => {
    if (timer !== undefined && pendingArgs !== undefined) {
      clock.clearTimeout(timer);
      timer = undefined;
      const a = pendingArgs;
      pendingArgs = undefined;
      lastFire = clock.now();
      fn(...a);
    }
  };

  wrapped.cancel = () => {
    if (timer !== undefined) clock.clearTimeout(timer);
    timer = undefined;
    pendingArgs = undefined;
  };

  return wrapped;
}
