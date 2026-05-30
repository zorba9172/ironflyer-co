import type { Theme } from '@mui/material/styles';
import type { EChartsOption } from '@ironflyer/ui-web/fx';

export type StudioChartDatum = {
  name: string;
  value: number;
  color?: string;
};

export type StudioLineSeries = {
  name: string;
  data: number[];
  color?: string;
  area?: boolean;
};

const asRecord = (value: unknown): Record<string, unknown> =>
  value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {};

const mergeRecord = (base: unknown, override: unknown): Record<string, unknown> => ({
  ...asRecord(base),
  ...asRecord(override),
});

const mergeAxis = (base: unknown, override: unknown) => {
  if (Array.isArray(override)) {
    const baseItems = Array.isArray(base) ? base : [];
    return override.map((item, index) => mergeRecord(baseItems[index], item));
  }
  return mergeRecord(base, override);
};

export function studioChartScaffold(theme: Theme): EChartsOption {
  const gridColor = theme.palette.mode === 'dark' ? theme.studio.chart.gridDark : theme.studio.chart.gridLight;
  return {
    color: [...theme.studio.chart.series],
    textStyle: {
      color: theme.palette.text.secondary,
      fontFamily: theme.typography.fontFamily,
    },
    tooltip: {
      trigger: 'item',
      backgroundColor: theme.palette.surfaceRaised,
      borderColor: theme.palette.divider,
      borderWidth: 1,
      padding: [10, 12],
      textStyle: { color: theme.palette.text.primary, fontSize: 12 },
      extraCssText: 'border-radius: 8px; box-shadow: 0 8px 24px rgba(17,24,39,0.08);',
    },
    legend: {
      bottom: 0,
      itemWidth: 9,
      itemHeight: 9,
      icon: 'circle',
      textStyle: { color: theme.palette.text.secondary, fontSize: 11, fontFamily: theme.typography.fontFamily },
    },
    grid: { left: 36, right: 18, top: 30, bottom: 32, containLabel: true },
    xAxis: {
      axisLabel: { color: theme.palette.text.disabled, fontSize: 10 },
      axisLine: { lineStyle: { color: theme.palette.divider } },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: gridColor } },
    },
    yAxis: {
      axisLabel: { color: theme.palette.text.disabled, fontSize: 10 },
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: gridColor } },
    },
  };
}

export function withStudioChartTheme(theme: Theme, option: EChartsOption): EChartsOption {
  const base = studioChartScaffold(theme);
  const o = option as Record<string, unknown>;
  return {
    ...base,
    ...option,
    tooltip: mergeRecord(base.tooltip, o.tooltip),
    legend: mergeRecord(base.legend, o.legend),
    grid: mergeRecord(base.grid, o.grid),
    xAxis: o.xAxis == null ? undefined : mergeAxis(base.xAxis, o.xAxis),
    yAxis: o.yAxis == null ? undefined : mergeAxis(base.yAxis, o.yAxis),
  };
}

export function donutOption(
  theme: Theme,
  {
    data,
    centerLabel,
    centerColor,
    emptyLabel = 'No data',
    radius = ['58%', '80%'],
  }: {
    data: StudioChartDatum[];
    centerLabel?: string;
    centerColor?: string;
    emptyLabel?: string;
    radius?: [string, string];
  },
): EChartsOption {
  const series = [...theme.studio.chart.series];
  const cleaned = data.filter((d) => d.value > 0);
  return {
    tooltip: { trigger: 'item' },
    legend: { bottom: 0, type: cleaned.length > 5 ? 'scroll' : undefined },
    series: [{
      type: 'pie',
      radius,
      avoidLabelOverlap: true,
      itemStyle: { borderColor: theme.palette.background.paper, borderWidth: 2 },
      label: centerLabel
        ? {
            show: true,
            position: 'center',
            formatter: centerLabel,
            color: centerColor ?? theme.palette.text.primary,
            fontSize: 22,
            lineHeight: 22,
            fontWeight: 800,
          }
        : { show: false },
      data: cleaned.length
        ? cleaned.map((d, index) => ({ value: d.value, name: d.name, itemStyle: { color: d.color ?? series[index % series.length] } }))
        : [{ value: 1, name: emptyLabel, itemStyle: { color: theme.palette.action.hover } }],
    }],
  };
}

export function gaugeOption(
  theme: Theme,
  {
    value,
    color,
    formatter,
    radius = '94%',
  }: {
    value: number;
    color?: string;
    formatter?: string;
    radius?: string;
  },
): EChartsOption {
  const track = theme.palette.mode === 'dark' ? theme.studio.chart.trackDark : theme.studio.chart.trackLight;
  const tone = color ?? theme.palette.primary.main;
  return {
    series: [{
      type: 'gauge',
      startAngle: 210,
      endAngle: -30,
      min: 0,
      max: 100,
      radius,
      progress: { show: true, width: 14, itemStyle: { color: tone } },
      axisLine: { lineStyle: { width: 14, color: [[1, track]] } },
      axisTick: { show: false },
      splitLine: { show: false },
      axisLabel: { show: false },
      pointer: { show: false },
      anchor: { show: false },
      detail: {
        valueAnimation: true,
        formatter: formatter ?? '{value}%',
        color: tone,
        fontSize: 30,
        fontWeight: 800,
        offsetCenter: [0, 0],
      },
      data: [{ value }],
    }],
  };
}

export function lineTrendOption(
  theme: Theme,
  {
    categories,
    series,
  }: {
    categories: string[];
    series: StudioLineSeries[];
  },
): EChartsOption {
  const tones = [...theme.studio.chart.series];
  return {
    tooltip: { trigger: 'axis' },
    legend: { top: 0, right: 0, bottom: undefined },
    xAxis: { type: 'category', boundaryGap: false, data: categories },
    yAxis: { type: 'value' },
    series: series.map((item, index) => {
      const tone = item.color ?? tones[index % tones.length];
      return {
        name: item.name,
        type: 'line',
        smooth: true,
        showSymbol: false,
        lineStyle: { width: 2.5, color: tone },
        itemStyle: { color: tone },
      areaStyle: item.area ? { opacity: 0.08, color: tone } : undefined,
        data: item.data,
      };
    }),
  };
}

export function horizontalBarOption(
  theme: Theme,
  {
    labels,
    values,
    colors,
  }: {
    labels: string[];
    values: number[];
    colors?: string[];
  },
): EChartsOption {
  const tones = colors?.length ? colors : [...theme.studio.chart.series];
  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: 8, right: 24, top: 12, bottom: 8, containLabel: true },
    xAxis: { type: 'value' },
    yAxis: { type: 'category', data: labels },
    series: [{
      type: 'bar',
      barWidth: '54%',
      data: values.map((value, index) => ({ value, itemStyle: { color: tones[index % tones.length], borderRadius: [0, 4, 4, 0] } })),
      label: { show: true, position: 'right', color: theme.palette.text.secondary, fontSize: 11 },
    }],
  };
}
