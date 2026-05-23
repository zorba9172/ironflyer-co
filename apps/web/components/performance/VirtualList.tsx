'use client';

import { useEffect, useMemo, useRef, useState, type Key, type ReactNode } from 'react';
import { Box, type SxProps, type Theme } from '@mui/material';

interface VirtualListProps<T> {
  items: readonly T[];
  itemHeight: number;
  getItemHeight?: (item: T, index: number) => number;
  height: number | string;
  overscan?: number;
  keyExtractor: (item: T, index: number) => Key;
  renderItem: (item: T, index: number) => ReactNode;
  sx?: SxProps<Theme>;
  ariaLabel?: string;
  itemOverflow?: 'hidden' | 'visible';
}

export function VirtualList<T>({
  items,
  itemHeight,
  getItemHeight,
  height,
  overscan = 6,
  keyExtractor,
  renderItem,
  sx,
  ariaLabel,
  itemOverflow = 'hidden',
}: VirtualListProps<T>) {
  const scrollerRef = useRef<HTMLDivElement | null>(null);
  const [scrollTop, setScrollTop] = useState(0);
  const [viewportHeight, setViewportHeight] = useState(
    typeof height === 'number' ? height : 480,
  );

  useEffect(() => {
    const node = scrollerRef.current;
    if (!node) return undefined;

    const updateHeight = () => {
      const next = node.clientHeight;
      if (next) {
        setViewportHeight((current) => (Math.abs(next - current) > 1 ? next : current));
      }
    };
    updateHeight();

    if (typeof ResizeObserver === 'undefined') return undefined;
    const observer = new ResizeObserver(updateHeight);
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  const metrics = useMemo(() => {
    if (!getItemHeight) {
      return {
        variable: false,
        totalHeight: items.length * itemHeight,
        heightAt: () => itemHeight,
        topAt: (index: number) => index * itemHeight,
        findStart: (top: number) => Math.floor(top / itemHeight),
      };
    }

    const offsets = new Array<number>(items.length + 1);
    offsets[0] = 0;
    for (let i = 0; i < items.length; i += 1) {
      offsets[i + 1] = offsets[i] + Math.max(1, getItemHeight(items[i], i));
    }

    return {
      variable: true,
      totalHeight: offsets[items.length],
      heightAt: (index: number) => offsets[index + 1] - offsets[index],
      topAt: (index: number) => offsets[index],
      findStart: (top: number) => {
        let lo = 0;
        let hi = offsets.length - 1;
        while (lo < hi) {
          const mid = Math.floor((lo + hi) / 2);
          if (offsets[mid] <= top) lo = mid + 1;
          else hi = mid;
        }
        return Math.max(0, lo - 1);
      },
    };
  }, [getItemHeight, itemHeight, items]);

  useEffect(() => {
    const maxScrollTop = Math.max(0, metrics.totalHeight - viewportHeight);
    if (scrollTop > maxScrollTop) {
      setScrollTop(maxScrollTop);
      if (scrollerRef.current) scrollerRef.current.scrollTop = maxScrollTop;
    }
  }, [metrics.totalHeight, scrollTop, viewportHeight]);

  const range = useMemo(() => {
    const rawStart = metrics.findStart(scrollTop);
    const start = Math.min(items.length, Math.max(0, rawStart - overscan));
    let end = Math.min(items.length, rawStart + overscan);
    const targetBottom = scrollTop + viewportHeight;
    while (end < items.length && metrics.topAt(end) < targetBottom) end += 1;
    end = Math.min(items.length, end + overscan);
    return { start, end };
  }, [items.length, metrics, overscan, scrollTop, viewportHeight]);

  const visible = items.slice(range.start, range.end);

  return (
    <Box
      role="list"
      aria-label={ariaLabel}
      sx={[
        {
          height,
          overflowY: 'auto',
          overflowX: 'hidden',
          position: 'relative',
          contain: 'strict',
        },
        ...(Array.isArray(sx) ? sx : sx ? [sx] : []),
      ]}
      ref={scrollerRef}
      onScroll={(event) => setScrollTop(event.currentTarget.scrollTop)}
    >
      <Box sx={{ height: metrics.totalHeight, position: 'relative' }}>
        {visible.map((item, offset) => {
          const index = range.start + offset;
          return (
            <Box
              role="listitem"
              key={keyExtractor(item, index)}
              sx={{
                position: 'absolute',
                top: metrics.topAt(index),
                left: 0,
                right: 0,
                height: metrics.heightAt(index),
                overflow: itemOverflow,
              }}
            >
              {renderItem(item, index)}
            </Box>
          );
        })}
      </Box>
    </Box>
  );
}
