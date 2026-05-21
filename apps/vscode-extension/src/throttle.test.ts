import { beforeEach, describe, expect, it, vi } from 'vitest';
import { throttleTrailing, ThrottleClock } from './throttle';

class FakeClock implements ThrottleClock {
  private current = 0;
  private nextHandle = 1;
  private timers = new Map<number, { fireAt: number; cb: () => void }>();

  now() { return this.current; }
  setTimeout(cb: () => void, ms: number) {
    const handle = this.nextHandle++;
    this.timers.set(handle, { fireAt: this.current + ms, cb });
    return handle;
  }
  clearTimeout(h: unknown) { this.timers.delete(h as number); }
  advance(ms: number) {
    this.current += ms;
    for (const [h, t] of [...this.timers.entries()]) {
      if (t.fireAt <= this.current) {
        this.timers.delete(h);
        t.cb();
      }
    }
  }
}

describe('throttleTrailing', () => {
  let clock: FakeClock;
  let fn: ReturnType<typeof vi.fn>;
  let throttled: ReturnType<typeof throttleTrailing>;

  beforeEach(() => {
    clock = new FakeClock();
    fn = vi.fn();
    throttled = throttleTrailing(fn as any, 100, clock);
  });

  it('fires leading-edge on the first call', () => {
    throttled('a');
    expect(fn).toHaveBeenCalledTimes(1);
    expect(fn).toHaveBeenCalledWith('a');
  });

  it('collapses a burst into a single trailing call with the last args', () => {
    throttled('a'); // leading fires
    clock.advance(10); throttled('b');
    clock.advance(10); throttled('c');
    clock.advance(10); throttled('d');
    expect(fn).toHaveBeenCalledTimes(1);
    clock.advance(100);
    expect(fn).toHaveBeenCalledTimes(2);
    expect(fn).toHaveBeenLastCalledWith('d');
  });

  it('fires immediately again once the window has elapsed', () => {
    throttled('a');
    clock.advance(150);
    throttled('b');
    expect(fn).toHaveBeenCalledTimes(2);
    expect(fn).toHaveBeenLastCalledWith('b');
  });

  it('flush() runs the pending trailing call immediately', () => {
    throttled('a');
    clock.advance(10); throttled('b');
    expect(fn).toHaveBeenCalledTimes(1);
    throttled.flush();
    expect(fn).toHaveBeenCalledTimes(2);
    expect(fn).toHaveBeenLastCalledWith('b');
  });

  it('cancel() drops the pending trailing call', () => {
    throttled('a');
    clock.advance(10); throttled('b');
    throttled.cancel();
    clock.advance(200);
    expect(fn).toHaveBeenCalledTimes(1);
  });
});
