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
import { useQuery } from '@tanstack/react-query'
import {
  Eye,
  LogIn,
  MapPin,
  TrendingUp,
  UserPlus,
  Users,
  Zap,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { StatCard } from '@/features/dashboard/components/ui/stat-card'
import { formatNumber } from '@/lib/format'

import { getStatisticsOverview } from './api'
import { StatChart } from './components/statistics-charts'
import { buildRegionSpec, buildTrendSpec } from './lib/charts'
import type { StatisticsOverview } from './types'

const EMPTY_OVERVIEW: StatisticsOverview = {
  today: {
    registrations: 0,
    logins: 0,
    visitors: 0,
    active_users: 0,
    call_count: 0,
  },
  region_distribution: [],
  trend: [],
}

const TREND_DAYS_OPTIONS = [7, 14, 30]

export function StatisticsPage() {
  const { t } = useTranslation()
  const [days, setDays] = useState(7)

  const query = useQuery({
    queryKey: ['statistics-overview', days],
    queryFn: async () => getStatisticsOverview(days),
  })

  const loading = query.isLoading
  const error = query.isError || query.data?.success === false
  const overview = query.data?.data ?? EMPTY_OVERVIEW
  const today = overview.today

  const trendSparkline = useMemo(
    () => ({
      active_users: overview.trend.map((p) => p.active_users),
      visitors: overview.trend.map((p) => p.visitors),
      call_count: overview.trend.map((p) => p.call_count),
    }),
    [overview.trend]
  )

  const trendSpec = useMemo(
    () => buildTrendSpec(overview.trend, t),
    [overview.trend, t]
  )
  const regionSpec = useMemo(
    () => buildRegionSpec(overview.region_distribution, t),
    [overview.region_distribution, t]
  )

  const daysSelect = (
    <NativeSelect
      value={String(days)}
      onChange={(e) => setDays(Number(e.target.value))}
      className='h-8 w-auto text-xs'
    >
      {TREND_DAYS_OPTIONS.map((d) => (
        <NativeSelectOption key={d} value={String(d)}>
          {t('Last {{count}} days', { count: d })}
        </NativeSelectOption>
      ))}
    </NativeSelect>
  )

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Statistics')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>{daysSelect}</SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-4'>
          <div className='grid grid-cols-2 gap-3 rounded-lg border p-3 sm:grid-cols-3 sm:gap-4 sm:p-4 lg:grid-cols-5'>
            <StatCard
              title={t('Active Users')}
              value={formatNumber(today.active_users)}
              description={t('Unique callers today')}
              icon={Users}
              tone='accent-1'
              sparkline={trendSparkline.active_users}
              sparklineVariant='line'
              loading={loading}
              error={error}
            />
            <StatCard
              title={t('Registrations')}
              value={formatNumber(today.registrations)}
              description={t('New sign-ups today')}
              icon={UserPlus}
              tone='accent-2'
              loading={loading}
              error={error}
            />
            <StatCard
              title={t('Logins')}
              value={formatNumber(today.logins)}
              description={t('Users logged in today')}
              icon={LogIn}
              tone='accent-3'
              loading={loading}
              error={error}
            />
            <StatCard
              title={t('Visitors')}
              value={formatNumber(today.visitors)}
              description={t('Unique visitors today')}
              icon={Eye}
              tone='accent-1'
              sparkline={trendSparkline.visitors}
              sparklineVariant='line'
              loading={loading}
              error={error}
            />
            <StatCard
              title={t('Call Count')}
              value={formatNumber(today.call_count)}
              description={t('API calls today')}
              icon={Zap}
              tone='accent-2'
              sparkline={trendSparkline.call_count}
              sparklineVariant='line'
              loading={loading}
              error={error}
            />
          </div>

          <div className='grid grid-cols-1 gap-4 lg:grid-cols-2'>
            <StatChart
              title={t('Recent Trend')}
              chartKey={`trend-${days}-${overview.trend.length}`}
              spec={trendSpec}
              empty={!loading && overview.trend.length === 0}
              emptyText={t('No data available')}
              action={
                <span className='text-muted-foreground flex items-center gap-1 text-xs'>
                  <TrendingUp className='size-3.5' />
                  {t('Last {{count}} days', { count: days })}
                </span>
              }
            />
            <StatChart
              title={t('IP Region Distribution')}
              chartKey={`region-${overview.region_distribution.length}`}
              spec={regionSpec}
              empty={!loading && overview.region_distribution.length === 0}
              emptyText={t('No data available')}
              action={
                <span className='text-muted-foreground flex items-center gap-1 text-xs'>
                  <MapPin className='size-3.5' />
                  {t('Today')}
                </span>
              }
            />
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
