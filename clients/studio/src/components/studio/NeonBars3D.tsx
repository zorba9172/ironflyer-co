import { useTheme } from '@mui/material/styles';
import { Bars3D, type Bar3DDatum } from '@ironflyer/ui-web/fx';

export type NeonBars3DProps = {
  data: Bar3DDatum[];
  height?: number;
  max?: number;
  rotate?: boolean;
};

// Studio-neon 3D bar field. Feeds the locked neon chart series into the lazy
// three.js Bars3D so every operator surface gets a data-bound 3D graph with
// zero color decisions at the call site. Bind `data` to real orchestrator
// state (cost shares, throughput, gate levels) — viz-first, never decoration.
export function NeonBars3D({ data, height = 300, max, rotate = true }: NeonBars3DProps) {
  const theme = useTheme();
  return <Bars3D data={data} colors={[...theme.studio.chart.series]} height={height} max={max} rotate={rotate} />;
}
