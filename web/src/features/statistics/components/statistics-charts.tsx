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
import { VChart } from '@visactor/react-vchart'
import { useEffect, useRef, useState, type ReactNode } from 'react'

import { useTheme } from '@/context/theme-provider'
import { VCHART_OPTION } from '@/lib/vchart'

let themeManagerPromise: Promise<
  (typeof import('@visactor/vchart'))['ThemeManager']
> | null = null

interface StatChartProps {
  title: string
  spec: Record<string, unknown> | null
  chartKey: string
  empty?: boolean
  emptyText?: string
  action?: ReactNode
}

// 复用 dashboard 的 VChart 主题就绪 + remount 处理，画统计看板的趋势/地区图。
export function StatChart(props: StatChartProps) {
  const { resolvedTheme } = useTheme()
  const [themeReady, setThemeReady] = useState(false)
  const themeManagerRef = useRef<
    (typeof import('@visactor/vchart'))['ThemeManager'] | null
  >(null)

  useEffect(() => {
    const updateTheme = async () => {
      setThemeReady(false)
      if (!themeManagerPromise) {
        themeManagerPromise = import('@visactor/vchart').then(
          (m) => m.ThemeManager
        )
      }
      const ThemeManager = await themeManagerPromise
      themeManagerRef.current = ThemeManager
      ThemeManager.setCurrentTheme(resolvedTheme === 'dark' ? 'dark' : 'light')
      setThemeReady(true)
    }
    updateTheme()
  }, [resolvedTheme])

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex w-full items-center justify-between gap-2 border-b px-3 py-2 sm:px-5 sm:py-3'>
        <div className='text-sm font-semibold'>{props.title}</div>
        {props.action}
      </div>
      <div className='h-[300px] p-1.5 sm:h-96 sm:p-2'>
        {props.empty ? (
          <div className='text-muted-foreground flex h-full items-center justify-center text-sm'>
            {props.emptyText}
          </div>
        ) : (
          themeReady &&
          props.spec && (
            <VChart
              key={`${props.chartKey}-${resolvedTheme}`}
              spec={{
                ...props.spec,
                theme: resolvedTheme === 'dark' ? 'dark' : 'light',
                background: 'transparent',
              }}
              option={VCHART_OPTION}
            />
          )
        )}
      </div>
    </div>
  )
}
