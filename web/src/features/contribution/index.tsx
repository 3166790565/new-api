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

import { ConfirmDialog } from '@/components/confirm-dialog'
import { StaticDataTable } from '@/components/data-table'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { formatTimestamp } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'

import {
  getSelfContributionShare,
  getSelfContributions,
  withdrawSelfContribution,
} from './api'
import { ContributionEarningsSheet } from './components/contribution-earnings-sheet'
import { ContributionStatusBadge } from './components/contribution-status-badge'
import { ContributionWizard } from './components/contribution-wizard'
import { CONTRIBUTION_PAGE_SIZE, CONTRIBUTION_STATUS } from './constants'
import type { Contribution } from './types'

export function ContributionPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editId, setEditId] = useState<number | undefined>(undefined)
  const [earningsOpen, setEarningsOpen] = useState(false)
  const [withdrawTarget, setWithdrawTarget] = useState<Contribution | null>(
    null
  )

  const listQuery = useQuery({
    queryKey: ['self-contributions', page],
    queryFn: async () =>
      getSelfContributions({ p: page, page_size: CONTRIBUTION_PAGE_SIZE }),
  })

  const shareQuery = useQuery({
    queryKey: ['self-contribution-share'],
    queryFn: async () => getSelfContributionShare(),
  })
  const sharePercent = shareQuery.data?.data?.share_percent

  const items = listQuery.data?.data?.items ?? []
  const total = listQuery.data?.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / CONTRIBUTION_PAGE_SIZE))

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ['self-contributions'] })

  const withdrawMutation = useMutation({
    mutationFn: async (id: number) => withdrawSelfContribution(id),
    onSuccess: (result) => {
      if (!result.success) {
        handleServerError(result)
        return
      }
      toast.success(t('Contribution withdrawn'))
      setWithdrawTarget(null)
      void refresh()
    },
    onError: (error) => handleServerError(error),
  })

  const openCreate = () => {
    setEditId(undefined)
    setDrawerOpen(true)
  }
  const openEdit = (id: number) => {
    setEditId(id)
    setDrawerOpen(true)
  }

  const columns = [
    {
      id: 'name',
      header: t('Name'),
      cell: (row: Contribution) => row.name,
    },
    {
      id: 'key',
      header: t('Key'),
      cell: (row: Contribution) => (
        <span className='font-mono text-xs'>{row.key}</span>
      ),
    },
    {
      id: 'group',
      header: t('Group'),
      cell: (row: Contribution) => row.group,
    },
    {
      id: 'status',
      header: t('Status'),
      cell: (row: Contribution) => (
        <ContributionStatusBadge status={row.status} />
      ),
    },
    {
      id: 'created_time',
      header: t('Created'),
      cell: (row: Contribution) => formatTimestamp(row.created_time),
    },
    {
      id: 'actions',
      header: t('Actions'),
      className: 'text-right',
      cellClassName: 'text-right',
      cell: (row: Contribution) => {
        if (row.status !== CONTRIBUTION_STATUS.PENDING) {
          return <span className='text-muted-foreground'>-</span>
        }
        return (
          <div className='flex justify-end gap-2'>
            <Button
              variant='outline'
              size='sm'
              onClick={() => openEdit(row.id)}
            >
              {t('Edit')}
            </Button>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setWithdrawTarget(row)}
            >
              {t('Withdraw')}
            </Button>
          </div>
        )
      },
    },
  ]

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t('My Contributions')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          {sharePercent !== undefined && (
            <span className='text-muted-foreground mr-1 text-sm'>
              {t('Current share')}:{' '}
              <span className='text-foreground font-semibold'>
                {sharePercent}%
              </span>
            </span>
          )}
          <Button variant='outline' onClick={() => setEarningsOpen(true)}>
            {t('Earnings')}
          </Button>
          <Button onClick={openCreate}>{t('Contribute a Channel')}</Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex flex-col gap-4'>
            <StaticDataTable<Contribution>
              columns={columns}
              data={items}
              getRowKey={(row) => row.id}
              emptyContent={
                listQuery.isLoading
                  ? t('Loading...')
                  : t('No contributions yet')
              }
            />
            {totalPages > 1 && (
              <div className='flex items-center justify-end gap-2'>
                <Button
                  variant='outline'
                  size='sm'
                  disabled={page <= 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                >
                  {t('Previous')}
                </Button>
                <span className='text-muted-foreground text-sm'>
                  {page} / {totalPages}
                </span>
                <Button
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
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <ContributionWizard
        open={drawerOpen}
        onOpenChange={setDrawerOpen}
        contributionId={editId}
        onSaved={refresh}
      />
      <ContributionEarningsSheet
        open={earningsOpen}
        onOpenChange={setEarningsOpen}
      />
      <ConfirmDialog
        open={withdrawTarget !== null}
        onOpenChange={(open) => {
          if (!open) setWithdrawTarget(null)
        }}
        title={t('Withdraw Contribution')}
        desc={t(
          'This will cancel the pending submission. You can submit it again later.'
        )}
        destructive
        isLoading={withdrawMutation.isPending}
        handleConfirm={() => {
          if (withdrawTarget) withdrawMutation.mutate(withdrawTarget.id)
        }}
        confirmText={t('Withdraw')}
      />
    </>
  )
}
