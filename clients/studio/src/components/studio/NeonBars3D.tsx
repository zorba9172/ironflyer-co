import { useMemo } from 'react';
import { useTheme } from '@mui/material/styles';
import type { Bar3DDatum } from '@ironflyer/ui-web/fx';
import { StudioChart, barOption } from '../charts';

export type { Bar3DDatum };

export type NeonBars3DProps = {
  data: Bar3DDatum[];
  height?: number;
  max?: number;
  /** retained for call-site compatibility; the 2D field never rotates */
  rotate?: boolean;
};

// Clean 2D categorical bar field (formerly a three.js Bars3D). Per the locked
// references the "provider spend" surface is a flat, colorful bar chart — not a
// rotating 3D object — so this renders the data through the shared StudioChart
// with the locked categorical palette. The name + props are kept so existing
// call sites stay untouched; `rotate`/`max` are accepted and ignored.
export function NeonBars3D({ data, height = 280 }: NeonBars3DProps) {
  const theme = useTheme();
  const option = useMemo(() => {
    const base = barOption(theme, {
      categories: data.map((d) => d.label),
      series: [{ name: 'value', data: data.map((d) => d.value) }],
      multicolor: true,
    });
    // Keep dense category axes legible: show every label, tilt when crowded.
    return {
      ...base,
      grid: { left: 8, right: 16, top: 16, bottom: 8, containLabel: true },
      xAxis: {
        type: 'category' as const,
        data: data.map((d) => d.label),
        axisLabel: {
          interval: 0,
          rotate: data.length > 7 ? 38 : 0,
          color: theme.palette.text.disabled,
          fontSize: 10,
          hideOverlap: true,
        },
      },
    };
  }, [theme, data]);

  return <StudioChart option={option} height={height} />;
}
