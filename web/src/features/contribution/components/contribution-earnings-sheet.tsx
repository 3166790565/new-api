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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StaticDataTable } from '@/components/data-table'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { toIntlLocale } from '@/i18n/languages'
import { formatQuotaWithCurrency } from '@/lib/currency'
import { formatTimestamp } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'

import {
  getSelfContributionEarnings,
  transferContributionEarnings,
} from '../api'
import { CONTRIBUTION_PAGE_SIZE } from '../constants'
import type { ContributionEarning } from '../types'

type ContributionEarningsSheetProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function ContributionEarningsSheet(
  props: ContributionEarningsSheetProps
) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)

  const earningsQuery = useQuery({
    queryKey: ['contribution-earnings', page],
    queryFn: async () =>
      getSelfContributionEarnings({
        p: page,
        page_size: CONTRIBUTION_PAGE_SIZE,
      }),
    enabled: props.open,
  })

  const data = earningsQuery.data?.data
  const balance = data?.contribution_quota ?? 0
  const lifetime = data?.lifetime_share_quota ?? 0
  const earnings = data?.page.items ?? []
  const total = data?.page.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / CONTRIBUTION_PAGE_SIZE))

  const transferMutation = useMutation({
    mutationFn: async () => transferContributionEarnings(balance),
    onSuccess: (result) => {
      if (!result.success) {
        handleServerError(result)
        return
      }
      toast.success(t('Transferred to wallet'))
      void queryClient.invalidateQueries({
        queryKey: ['contribution-earnings'],
      })
    },
    onError: (error) => handleServerError(error),
  })

  const columns = [
    {
      id: 'created_time',
      header: t('Time'),
      cell: (row: ContributionEarning) => formatTimestamp(row.created_time),
    },
    {
      id: 'model_name',
      header: t('Model'),
      cell: (row: ContributionEarning) => row.model_name,
    },
    {
      id: 'share_percent',
      header: t('Share'),
      cell: (row: ContributionEarning) => `${row.share_percent}%`,
    },
    {
      id: 'share_quota',
      header: t('Earned'),
      cell: (row: ContributionEarning) =>
        formatQuotaWithCurrency(row.share_quota),
    },
  ]

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className='flex w-full flex-col gap-0 sm:max-w-[640px]'>
        <SheetHeader>
          <SheetTitle>{t('Contribution Earnings')}</SheetTitle>
          <SheetDescription>
            {t('Your share balance and the per-request earning history.')}
          </SheetDescription>
        </SheetHeader>

        <div className='flex flex-col gap-4 overflow-y-auto px-4 py-4'>
          <div className='grid grid-cols-2 gap-3'>
            <div className='rounded-lg border p-3'>
              <div className='text-muted-foreground text-sm'>
                {t('Available Balance')}
              </div>
              <div className='mt-1 text-lg font-semibold'>
                {formatQuotaWithCurrency(balance)}
              </div>
            </div>
            <div className='rounded-lg border p-3'>
              <div className='text-muted-foreground text-sm'>
                {t('Lifetime Earnings')}
              </div>
              <div className='mt-1 text-lg font-semibold'>
                {formatQuotaWithCurrency(lifetime)}
              </div>
            </div>
          </div>

          <Button
            type='button'
            disabled={balance <= 0 || transferMutation.isPending}
            onClick={() => transferMutation.mutate()}
          >
            {transferMutation.isPending
              ? t('Transferring...')
              : t('Transfer all to wallet')}
          </Button>

          <StaticDataTable<ContributionEarning>
            columns={columns}
            data={earnings}
            getRowKey={(row) => row.id}
            emptyContent={t('No earnings yet')}
          />

          {totalPages > 1 && (
            <div className='flex items-center justify-end gap-2'>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={page <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
              >
                {t('Previous')}
              </Button>
              <span className='text-muted-foreground text-sm'>
                {new Intl.NumberFormat(locale).format(page)} /{' '}
                {new Intl.NumberFormat(locale).format(totalPages)}
              </span>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={page >= totalPages}
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              >
                {t('Next')}
              </Button>
            </div>
          )}
        </div>

        <SheetFooter>
          <SheetClose render={<Button variant='outline' />}>
            {t('Close')}
          </SheetClose>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
