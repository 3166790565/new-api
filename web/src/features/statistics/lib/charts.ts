/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { RegionCount, TrendPoint } from '../types'

type TFunction = (key: string) => string

const TREND_COLORS = ['#5B8FF9', '#5AD8A6', '#F6BD16']

const formatInt = (value: number) =>
  Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(
    Number(value) || 0
  )

// 把趋势数据展开为 {Time, Metric, Count} 的多系列面积图数据。
export function buildTrendSpec(trend: TrendPoint[], t: TFunction) {
  const metrics: Array<{ key: keyof TrendPoint; label: string }> = [
    { key: 'active_users', label: t('Active Users') },
    { key: 'visitors', label: t('Visitors') },
    { key: 'call_count', label: t('Call Count') },
  ]

  const values: Array<{ Time: string; Metric: string; Count: number }> = []
  trend.forEach((point) => {
    metrics.forEach((metric) => {
      values.push({
        Time: point.day,
        Metric: metric.label,
        Count: Number(point[metric.key]) || 0,
      })
    })
  })

  return {
    type: 'area',
    data: [{ id: 'trendData', values }],
    xField: 'Time',
    yField: 'Count',
    seriesField: 'Metric',
    stack: false,
    legends: { visible: true, selectMode: 'multiple' },
    color: {
      type: 'ordinal',
      domain: metrics.map((m) => m.label),
      range: TREND_COLORS,
    },
    axes: [
      { orient: 'bottom', type: 'band' },
      {
        orient: 'left',
        type: 'linear',
        label: { formatMethod: (value: number) => formatInt(value) },
      },
    ],
    tooltip: {
      mark: {
        content: [
          {
            key: (datum: Record<string, unknown>) => datum?.Metric,
            value: (datum: Record<string, unknown>) =>
              formatInt(Number(datum?.Count) || 0),
          },
        ],
      },
      dimension: {
        content: [
          {
            key: (datum: Record<string, unknown>) => datum?.Metric,
            value: (datum: Record<string, unknown>) =>
              formatInt(Number(datum?.Count) || 0),
          },
        ],
      },
    },
    area: { style: { fillOpacity: 0.12, curveType: 'monotone' } },
    line: { style: { lineWidth: 2, curveType: 'monotone' } },
    point: { visible: false },
    background: { fill: 'transparent' },
    animation: true,
  }
}

// IP 地区分布：横向条形图，按人数降序（数据已由后端排序）。
export function buildRegionSpec(regions: RegionCount[], t: TFunction) {
  const values = regions.map((r) => ({
    Region: r.region || t('Unknown'),
    Count: Number(r.count) || 0,
  }))

  return {
    type: 'bar',
    data: [{ id: 'regionData', values }],
    xField: 'Count',
    yField: 'Region',
    seriesField: 'Region',
    direction: 'horizontal',
    legends: { visible: false },
    color: { type: 'ordinal', range: TREND_COLORS },
    label: {
      visible: true,
      position: 'outside',
      formatMethod: (value: number) => formatInt(value),
      style: { fontSize: 11 },
    },
    axes: [
      { orient: 'left', type: 'band' },
      { orient: 'bottom', type: 'linear', visible: false },
    ],
    tooltip: {
      mark: {
        content: [
          {
            key: (datum: Record<string, unknown>) => datum?.Region,
            value: (datum: Record<string, unknown>) =>
              formatInt(Number(datum?.Count) || 0),
          },
        ],
      },
    },
    bar: { state: { hover: { stroke: '#000', lineWidth: 1 } } },
    background: { fill: 'transparent' },
    animation: true,
  }
}
