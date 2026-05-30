import { useMemo } from 'react';
import { useTheme } from '@mui/material/styles';
import { StudioChart, gaugeOption } from '../charts';

export type GaugeRingProps = {
  /** 0..100 */
  value: number;
  /** neon tone for the progress arc; defaults to primary violet */
  color?: string;
  /** center detail formatter, e.g. '{value}%' or 'Ready' */
  formatter?: string;
  height?: number;
};

// The production-readiness dial (reference: Performance Review 72% gauge),
// wrapped as a one-prop primitive over the themed gaugeOption. Lazy echarts via
// StudioChart; color comes from the neon palette, never inline.
export function GaugeRing({ value, color, formatter, height = 168 }: GaugeRingProps) {
  const theme = useTheme();
  const option = useMemo(
    () => gaugeOption(theme, { value, color, formatter }),
    [theme, value, color, formatter],
  );
  return <StudioChart option={option} height={height} />;
}
