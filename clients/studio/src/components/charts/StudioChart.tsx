import { useMemo } from 'react';
import { Box } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { Chart, type EChartsOption } from '@ironflyer/ui-web/fx';
import { withStudioChartTheme } from './chartOptions';

export type StudioChartProps = {
  option: EChartsOption;
  height?: number | string;
};

export function StudioChart({ option, height = 280 }: StudioChartProps) {
  const theme = useTheme();
  const themedOption = useMemo(() => withStudioChartTheme(theme, option), [theme, option]);

  return (
    <Box
      sx={{
        width: '100%',
        minWidth: 0,
        '& canvas': { borderRadius: 2 },
      }}
    >
      <Chart option={themedOption} height={height} />
    </Box>
  );
}
