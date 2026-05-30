import { useTheme } from '@mui/material/styles';
import { Constellation3D, type Constellation3DNode, type Constellation3DLink } from '@ironflyer/ui-web/fx';

export type NeonConstellation3DProps = {
  nodes: Constellation3DNode[];
  links: Constellation3DLink[];
  height?: number;
  rotate?: boolean;
};

// Studio-neon 3D node constellation. Feeds the locked neon series into the lazy
// three.js Constellation3D — use it to render agent teams, gate graphs, or
// dependency maps as a living 3D object. Every node/edge must map to real
// state (an agent, a gate, a relationship), never abstract decoration.
export function NeonConstellation3D({ nodes, links, height = 320, rotate = true }: NeonConstellation3DProps) {
  const theme = useTheme();
  return (
    <Constellation3D nodes={nodes} links={links} colors={[...theme.studio.chart.series]} height={height} rotate={rotate} />
  );
}
