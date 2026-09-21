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
import type { ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'

import { MaskedValueDisplay } from '@/components/masked-value-display'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { Checkbox } from '@/components/ui/checkbox'
import { formatTimestampToDate } from '@/lib/format'

import { REGISTRATION_CODE_STATUSES } from '../constants'
import {
  formatRegistrationCodeValue,
  isRegistrationCodeExpired,
  isCodeExhausted,
  maskRegistrationCodeValue,
} from '../lib'
import type { RegistrationCode } from '../types'
import { DataTableRowActions } from './registration-codes-row-actions'

export function useRegistrationCodesColumns(): ColumnDef<RegistrationCode>[] {
  const { t } = useTranslation()
  return [
    {
      id: 'select',
      header: ({ table }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected()}
          indeterminate={table.getIsSomePageRowsSelected()}
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label={t('Select all')}
          className='translate-y-[2px]'
        />
      ),
      cell: ({ row }) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(!!value)}
          aria-label={t('Select row')}
          className='translate-y-[2px]'
        />
      ),
      enableSorting: false,
      enableHiding: false,
      size: 40,
    },
    {
      accessorKey: 'id',
      header: t('ID'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <TableId value={row.getValue('id') as number} className='w-[60px]' />
      ),
      size: 80,
    },
    {
      accessorKey: 'name',
      header: t('Name'),
      meta: { mobileTitle: true },
      cell: ({ row }) => (
        <span className='font-medium'>{row.getValue('name')}</span>
      ),
      size: 180,
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      meta: { mobileBadge: true },
      cell: ({ row }) => {
        const code = row.original
        const statusValue = row.getValue('status') as number
        if (isRegistrationCodeExpired(code.expired_time, statusValue)) {
          return (
            <StatusBadge
              label={t('Expired')}
              variant='warning'
              copyable={false}
              className='-ml-1.5'
            />
          )
        }
        if (isCodeExhausted(code.used_count, code.max_uses)) {
          return (
            <StatusBadge
              label={t('Used up')}
              variant='neutral'
              copyable={false}
              className='-ml-1.5'
            />
          )
        }
        const config = REGISTRATION_CODE_STATUSES[statusValue]
        if (!config) return null
        return (
          <StatusBadge
            label={t(config.labelKey)}
            variant={config.variant}
            copyable={false}
            className='-ml-1.5'
          />
        )
      },
      size: 110,
    },
    {
      id: 'code',
      accessorKey: 'code',
      header: t('Registration Code'),
      cell: function CodeCell({ row }) {
        const code = row.original
        const fullValue = formatRegistrationCodeValue(code)
        return (
          <MaskedValueDisplay
            label={t('Full Registration Code')}
            fullValue={fullValue}
            maskedValue={maskRegistrationCodeValue(fullValue)}
            copyTooltip={t('Copy code')}
            copyAriaLabel={t('Copy registration code')}
          />
        )
      },
      enableSorting: false,
      size: 320,
    },
    {
      id: 'usage',
      header: t('Usage'),
      cell: ({ row }) => {
        const code = row.original
        const unlimited = code.max_uses === 0
        const label = unlimited
          ? t('Unlimited')
          : `${code.used_count} / ${code.max_uses}`
        return (
          <StatusBadge
            label={label}
            variant='neutral'
            copyable={false}
            className='-ml-1.5'
          />
        )
      },
      size: 120,
    },
    {
      accessorKey: 'expired_time',
      header: t('Expires'),
      meta: { mobileHidden: true },
      cell: ({ row }) => {
        const expiredTime = row.getValue('expired_time') as number
        if (expiredTime === 0) {
          return (
            <StatusBadge
              label={t('Never')}
              variant='neutral'
              copyable={false}
              className='-ml-1.5'
            />
          )
        }
        return (
          <div className='min-w-[160px] font-mono text-sm'>
            {formatTimestampToDate(expiredTime)}
          </div>
        )
      },
      size: 180,
    },
    {
      accessorKey: 'created_time',
      header: t('Created'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <div className='min-w-[160px] font-mono text-sm'>
          {formatTimestampToDate(row.getValue('created_time'))}
        </div>
      ),
      size: 180,
    },
    {
      id: 'actions',
      header: () => t('Actions'),
      cell: ({ row }) => <DataTableRowActions row={row} />,
      meta: { pinned: 'right' as const },
    },
  ]
}
