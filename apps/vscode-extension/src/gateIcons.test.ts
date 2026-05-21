import { describe, expect, it } from 'vitest';
import {
  gateColorId,
  gateIcon,
  issueColorId,
  issueIcon,
  summarizeGates,
} from './gateIcons';
import type { GateState } from './api';

describe('gateIcon', () => {
  it('maps each known status to a non-empty codicon name', () => {
    const statuses = ['pending', 'running', 'passed', 'failed', 'blocked', 'repaired'] as const;
    for (const s of statuses) {
      expect(gateIcon(s)).toBeTruthy();
    }
  });

  it('uses pass-filled for passed and error for failed', () => {
    expect(gateIcon('passed')).toBe('pass-filled');
    expect(gateIcon('failed')).toBe('error');
  });
});

describe('gateColorId', () => {
  it('returns charts.green only for passed', () => {
    expect(gateColorId('passed')).toBe('charts.green');
    expect(gateColorId('failed')).toBe('charts.red');
    expect(gateColorId('pending')).toBeUndefined();
  });
});

describe('issueIcon / issueColorId', () => {
  it('escalates critical to the same icon family as error', () => {
    expect(issueIcon('critical')).toBe(issueIcon('error'));
    expect(issueColorId('critical')).toBe('charts.red');
  });

  it('warning is orange and info is blue', () => {
    expect(issueColorId('warning')).toBe('charts.orange');
    expect(issueColorId('info')).toBe('charts.blue');
  });
});

describe('summarizeGates', () => {
  const g = (status: GateState['status']): GateState => ({
    name: 'spec',
    status,
    updatedAt: '2026-05-22T00:00:00Z',
  });

  it('reports "no gates" for empty input', () => {
    expect(summarizeGates([])).toBe('no gates');
  });

  it('leads with "passed / total"', () => {
    expect(summarizeGates([g('passed'), g('passed'), g('failed')])).toBe(
      '2 / 3 passed · 1 failed',
    );
  });

  it('omits zero counts but always shows the passed/total leader', () => {
    expect(summarizeGates([g('pending'), g('pending')])).toBe('0 / 2 passed');
  });

  it('joins multiple non-zero buckets in priority order', () => {
    const s = summarizeGates([
      g('passed'), g('failed'), g('blocked'), g('running'), g('repaired'),
    ]);
    expect(s).toContain('1 failed');
    expect(s).toContain('1 blocked');
    expect(s).toContain('1 running');
    expect(s).toContain('1 repaired');
  });
});
