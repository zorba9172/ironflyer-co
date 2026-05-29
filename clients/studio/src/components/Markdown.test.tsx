import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Markdown, safeMarkdownUrl } from './Markdown';

describe('safeMarkdownUrl', () => {
  it('allows http, https, mailto links, and local anchors', () => {
    expect(safeMarkdownUrl('https://example.com/a')).toBe('https://example.com/a');
    expect(safeMarkdownUrl('http://example.com/a')).toBe('http://example.com/a');
    expect(safeMarkdownUrl('mailto:security@example.com')).toBe('mailto:security@example.com');
    expect(safeMarkdownUrl('#finding')).toBe('#finding');
    expect(safeMarkdownUrl('/docs/security')).toBe('/docs/security');
  });

  it('blocks scriptable or embedded payload URLs', () => {
    expect(safeMarkdownUrl('javascript:alert(1)')).toBeUndefined();
    expect(safeMarkdownUrl('data:text/html,<svg onload=alert(1)>')).toBeUndefined();
    expect(safeMarkdownUrl('mailto:security@example.com', 'image')).toBeUndefined();
  });
});

describe('Markdown', () => {
  it('does not render unsafe hrefs from agent markdown', () => {
    render(<Markdown>{'[bad](javascript:alert(1)) [good](https://example.com)'}</Markdown>);

    expect(screen.getByText('bad').closest('a')).not.toHaveAttribute('href');
    expect(screen.getByRole('link', { name: 'good' })).toHaveAttribute('href', 'https://example.com/');
  });
});
