import { describe, expect, it } from 'vitest';
import { drainFrames, parseFrame } from './sse';

describe('parseFrame', () => {
  it('parses a typed JSON event', () => {
    const evt = parseFrame('event: text\ndata: {"text":"hi"}');
    expect(evt).toEqual({ event: 'text', data: { text: 'hi' } });
  });

  it('defaults the event name to "message" when omitted', () => {
    expect(parseFrame('data: {"k":1}')).toEqual({ event: 'message', data: { k: 1 } });
  });

  it('falls back to raw string when data is not JSON', () => {
    expect(parseFrame('event: log\ndata: hello world')).toEqual({
      event: 'log',
      data: 'hello world',
    });
  });

  it('drops comment lines (heartbeats)', () => {
    expect(parseFrame(': ping')).toBeUndefined();
  });

  it('concatenates multi-line data', () => {
    const evt = parseFrame('event: text\ndata: line one\ndata: line two');
    expect(evt).toEqual({ event: 'text', data: 'line one\nline two' });
  });

  it('returns undefined when no data lines are present', () => {
    expect(parseFrame('event: noop')).toBeUndefined();
  });
});

describe('drainFrames', () => {
  it('parses multiple frames and keeps the trailing partial', () => {
    const buf = [
      'event: start\ndata: {"turn":"1"}\n',
      '\n',
      'event: text\ndata: {"text":"hello"}\n',
      '\n',
      'event: text\ndata: {"text":"partial', // unterminated tail
    ].join('');
    const { events, rest } = drainFrames(buf);
    expect(events).toHaveLength(2);
    expect(events[0]).toEqual({ event: 'start', data: { turn: '1' } });
    expect(events[1]).toEqual({ event: 'text', data: { text: 'hello' } });
    expect(rest).toBe('event: text\ndata: {"text":"partial');
  });

  it('handles an empty buffer', () => {
    expect(drainFrames('')).toEqual({ events: [], rest: '' });
  });

  it('handles a heartbeat-only frame', () => {
    const { events, rest } = drainFrames(': ping\n\n');
    expect(events).toEqual([]);
    expect(rest).toBe('');
  });
});
