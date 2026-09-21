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
import type { Table as TanstackTable } from '@tanstack/react-table'
import { Database } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { DISABLED_ROW_MOBILE } from '@/components/data-table'
import { MaskedValueDisplay } from '@/components/masked-value-display'
import { StatusBadge } from '@/components/status-badge'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

import { REGISTRATION_CODE_STATUSES } from '../constants'
import {
  formatRegistrationCodeValue,
  isRegistrationCodeExpired,
  isCodeExhausted,
  maskRegistrationCodeValue,
} from '../lib'
import type { RegistrationCode } from '../types'
import { DataTableRowActions } from './registration-codes-row-actions'

const MOBILE_SKELETON_KEYS = [
  'registration-code-mobile-skeleton-1',
  'registration-code-mobile-skeleton-2',
  'registration-code-mobile-skeleton-3',
  'registration-code-mobile-skeleton-4',
  'registration-code-mobile-skeleton-5',
]

function RegistrationCodesMobileSkeleton() {
  return (
    <div className='divide-border overflow-hidden rounded-lg border'>
      {MOBILE_SKELETON_KEYS.map((key) => (
        <div
          key={key}
          className='space-y-2 border-b px-3 py-2.5 last:border-b-0'
        >
          <div className='flex items-center justify-between'>
            <Skeleton className='h-4 w-32' />
            <Skeleton className='h-5 w-16 rounded-md' />
          </div>
          <Skeleton className='h-4 w-full' />
        </div>
      ))}
    </div>
  )
}

export function RegistrationCodesMobileList({
  table,
  isLoading,
}: {
  table: TanstackTable<RegistrationCode>
  isLoading: boolean
}) {
  const { t } = useTranslation()
  const rows = table.getRowModel().rows

  if (isLoading) {
    return <RegistrationCodesMobileSkeleton />
  }

  if (rows.length === 0) {
    return (
      <Empty>
        <EmptyMedia>
          <Database className='text-muted-foreground' />
        </EmptyMedia>
        <EmptyHeader>
          <EmptyTitle>{t('No Registration Codes Found')}</EmptyTitle>
          <EmptyDescription>
            {t(
              'No registration codes available. Create your first registration code to get started.'
            )}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  return (
    <div className='divide-border overflow-hidden rounded-lg border'>
      {rows.map((row) => {
        const code = row.original
        const statusConfig = REGISTRATION_CODE_STATUSES[code.status]
        const fullValue = formatRegistrationCodeValue(code)
        const isDisabled = isDisabledRow(code)

        let badgeLabel = ''
        let badgeVariant: 'neutral' | 'success' | 'warning' = 'neutral'
        if (statusConfig) {
          badgeLabel = t(statusConfig.labelKey)
          badgeVariant =
            statusConfig.variant === 'success' ? 'success' : 'neutral'
        } else if (isRegistrationCodeExpired(code.expired_time, code.status)) {
          badgeLabel = t('Expired')
          badgeVariant = 'warning'
        } else {
          badgeLabel = t('Used up')
        }
        const statusBadge = (
          <StatusBadge
            label={badgeLabel}
            variant={badgeVariant}
            copyable={false}
          />
        )

        return (
          <div
            key={code.id}
            className={cn(
              'space-y-2 border-b px-3 py-2.5 last:border-b-0',
              isDisabled && DISABLED_ROW_MOBILE
            )}
          >
            <div className='flex items-center justify-between gap-2'>
              <div className='min-w-0 flex-1'>
                <div className='flex items-center gap-2'>
                  <span className='truncate font-medium'>{code.name}</span>
                </div>
              </div>
              {statusBadge}
            </div>
            <MaskedValueDisplay
              label={t('Full Registration Code')}
              fullValue={fullValue}
              maskedValue={maskRegistrationCodeValue(fullValue)}
              copyTooltip={t('Copy code')}
              copyAriaLabel={t('Copy registration code')}
            />
            <div className='flex items-center justify-between'>
              <span className='text-muted-foreground text-sm'>
                {code.max_uses === 0
                  ? t('Unlimited')
                  : t('{{used}} / {{max}} uses', {
                      used: code.used_count,
                      max: code.max_uses,
                    })}
              </span>
              <DataTableRowActions row={row} />
            </div>
          </div>
        )
      })}
    </div>
  )
}

function isDisabledRow(code: RegistrationCode) {
  return (
    code.status !== 1 ||
    isRegistrationCodeExpired(code.expired_time, code.status) ||
    isCodeExhausted(code.used_count, code.max_uses)
  )
}
