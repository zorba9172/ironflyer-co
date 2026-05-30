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
      extraCssText: `border-radius: ${theme.studio.radius.sm}px; box-shadow: ${theme.shadows[6]};`,
    },
    legend: {
      bottom: 0,
      itemWidth: 8,
      itemHeight: 8,
      itemGap: 14,
      icon: 'circle',
      textStyle: { color: theme.palette.text.secondary, fontSize: 11, fontFamily: theme.typography.fontFamily },
    },
    grid: { left: 28, right: 16, top: 24, bottom: 28, containLabel: true },
    xAxis: {
      axisLabel: { color: theme.palette.text.disabled, fontSize: 11, margin: 10 },
      axisLine: { lineStyle: { color: theme.palette.divider } },
      axisTick: { show: false },
      splitLine: { show: false, lineStyle: { color: gridColor } },
    },
    yAxis: {
      axisLabel: { color: theme.palette.text.disabled, fontSize: 11 },
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: gridColor, type: 'dashed' } },
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
    radius = ['62%', '84%'],
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
  const isEmpty = cleaned.length === 0;
  return {
    tooltip: { trigger: 'item' },
    legend: { bottom: 0, itemGap: 14, type: cleaned.length > 5 ? 'scroll' : undefined },
    series: [{
      type: 'pie',
      radius,
      center: ['50%', '46%'],
      avoidLabelOverlap: true,
      padAngle: isEmpty ? 0 : 2,
      itemStyle: { borderColor: theme.palette.background.paper, borderWidth: 2, borderRadius: 3 },
      label: centerLabel
        ? {
            show: true,
            position: 'center',
            formatter: centerLabel,
            color: isEmpty ? theme.palette.text.disabled : (centerColor ?? theme.palette.text.primary),
            fontSize: 24,
            lineHeight: 24,
            fontWeight: 800,
          }
        : { show: false },
      data: isEmpty
        ? [{ value: 1, name: emptyLabel, itemStyle: { color: theme.palette.action.hover } }]
        : cleaned.map((d, index) => ({ value: d.value, name: d.name, itemStyle: { color: d.color ?? series[index % series.length] } })),
    }],
  };
}

export function gaugeOption(
  theme: Theme,
  {
    value,
    color,
    formatter,
    radius = '92%',
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
      center: ['50%', '54%'],
      progress: { show: true, width: 11, roundCap: true, itemStyle: { color: tone } },
      axisLine: { roundCap: true, lineStyle: { width: 11, color: [[1, track]] } },
      axisTick: { show: false },
      splitLine: { show: false },
      axisLabel: { show: false },
      pointer: { show: false },
      anchor: { show: false },
      detail: {
        valueAnimation: true,
        formatter: formatter ?? '{value}%',
        color: tone,
        fontSize: 26,
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
  const single = series.length <= 1;
  return {
    tooltip: { trigger: 'axis' },
    legend: single ? { show: false } : { top: 0, right: 0, itemGap: 14, bottom: undefined },
    grid: { top: single ? 12 : 28, bottom: 4 },
    xAxis: { type: 'category', boundaryGap: false, data: categories },
    yAxis: { type: 'value' },
    series: series.map((item, index) => {
      const tone = item.color ?? tones[index % tones.length];
      return {
        name: item.name,
        type: 'line',
        smooth: 0.4,
        showSymbol: false,
        symbolSize: 6,
        lineStyle: { width: 2.25, color: tone },
        itemStyle: { color: tone },
        areaStyle: item.area ? { opacity: 0.1, color: tone } : undefined,
        data: item.data,
      };
    }),
  };
}

export function pieOption(
  theme: Theme,
  {
    data,
    radius = '70%',
    roseType = false,
  }: {
    data: StudioChartDatum[];
    radius?: string;
    roseType?: boolean;
  },
): EChartsOption {
  const series = [...theme.studio.chart.series];
  const cleaned = data.filter((d) => d.value > 0);
  return {
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    legend: { bottom: 0, itemGap: 14, type: cleaned.length > 6 ? 'scroll' : undefined },
    series: [{
      type: 'pie',
      radius,
      roseType: roseType ? 'radius' : undefined,
      itemStyle: { borderColor: theme.palette.background.paper, borderWidth: 2, borderRadius: 4 },
      label: { color: theme.palette.text.secondary, fontSize: 11 },
      data: cleaned.map((d, i) => ({ value: d.value, name: d.name, itemStyle: { color: d.color ?? series[i % series.length] } })),
    }],
  };
}

export function barOption(
  theme: Theme,
  {
    categories,
    series,
    multicolor = false,
  }: {
    categories: string[];
    // one series → single bar set; many → grouped bars
    series: StudioLineSeries[];
    // when a single series, tint each bar from the categorical palette
    multicolor?: boolean;
  },
): EChartsOption {
  const tones = [...theme.studio.chart.series];
  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    legend: series.length > 1 ? { top: 0, right: 0, itemGap: 14 } : { show: false },
    grid: { top: series.length > 1 ? 28 : 12, bottom: 4 },
    xAxis: { type: 'category', data: categories },
    yAxis: { type: 'value' },
    series: series.map((s, si) => ({
      name: s.name,
      type: 'bar',
      barMaxWidth: 26,
      itemStyle: { borderRadius: [4, 4, 0, 0], color: s.color ?? tones[si % tones.length] },
      data: multicolor && series.length === 1
        ? s.data.map((value, i) => ({ value, itemStyle: { color: tones[i % tones.length], borderRadius: [4, 4, 0, 0] } }))
        : s.data,
    })),
  };
}

export function stackedBarOption(
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
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    legend: { top: 0, right: 0, itemGap: 14 },
    grid: { top: 28, bottom: 4 },
    xAxis: { type: 'category', data: categories },
    yAxis: { type: 'value' },
    series: series.map((s, si) => ({
      name: s.name,
      type: 'bar',
      stack: 'total',
      barMaxWidth: 30,
      itemStyle: { color: s.color ?? tones[si % tones.length] },
      emphasis: { focus: 'series' },
      data: s.data,
    })),
  };
}

export function radarOption(
  theme: Theme,
  {
    indicators,
    series,
  }: {
    indicators: { name: string; max: number }[];
    series: { name: string; data: number[]; color?: string }[];
  },
): EChartsOption {
  const tones = [...theme.studio.chart.series];
  const split = theme.palette.mode === 'dark' ? theme.studio.chart.gridDark : theme.studio.chart.gridLight;
  return {
    tooltip: { trigger: 'item' },
    legend: { bottom: 0 },
    radar: {
      indicator: indicators,
      splitNumber: 4,
      axisName: { color: theme.palette.text.secondary, fontSize: 11 },
      splitLine: { lineStyle: { color: split } },
      splitArea: { show: false },
      axisLine: { lineStyle: { color: split } },
    },
    series: [{
      type: 'radar',
      data: series.map((s, i) => {
        const tone = s.color ?? tones[i % tones.length];
        return { name: s.name, value: s.data, areaStyle: { opacity: 0.12, color: tone }, lineStyle: { color: tone, width: 2 }, itemStyle: { color: tone } };
      }),
    }],
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
