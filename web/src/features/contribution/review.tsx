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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { formatQuotaWithCurrency } from '@/lib/currency'
import { formatTimestamp } from '@/lib/format'

import {
  getContributionStats,
  getContributionsForReview,
  getContributors,
} from './api'
import { ContributionReviewSheet } from './components/contribution-review-sheet'
import { ContributionStatusBadge } from './components/contribution-status-badge'
import { ContributorShareDialog } from './components/contributor-share-dialog'
import {
  CONTRIBUTION_PAGE_SIZE,
  CONTRIBUTION_STATUS,
  CONTRIBUTION_STATUS_CONFIG,
  REVIEW_STATUS_FILTERS,
} from './constants'
import type { Contribution, Contributor } from './types'

function StatCard(props: { label: string; value: number }) {
  return (
    <div className='rounded-lg border p-3'>
      <div className='text-muted-foreground text-sm'>{props.label}</div>
      <div className='mt-1 text-lg font-semibold'>{props.value}</div>
    </div>
  )
}

function ReviewQueue() {
  const { t } = useTranslation()
  const [status, setStatus] = useState<number>(CONTRIBUTION_STATUS.PENDING)
  const [page, setPage] = useState(1)
  const [reviewId, setReviewId] = useState<number | null>(null)

  const listQuery = useQuery({
    queryKey: ['review-contributions', status, page],
    queryFn: async () =>
      getContributionsForReview(status, {
        p: page,
        page_size: CONTRIBUTION_PAGE_SIZE,
      }),
  })

  const items = listQuery.data?.data?.items ?? []
  const total = listQuery.data?.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / CONTRIBUTION_PAGE_SIZE))

  const columns = [
    {
      id: 'id',
      header: t('ID'),
      cell: (row: Contribution) => `#${row.id}`,
    },
    {
      id: 'name',
      header: t('Name'),
      cell: (row: Contribution) => row.name,
    },
    {
      id: 'user_id',
      header: t('User'),
      cell: (row: Contribution) => row.user_id,
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
      cell: (row: Contribution) => (
        <Button variant='outline' size='sm' onClick={() => setReviewId(row.id)}>
          {t('Review')}
        </Button>
      ),
    },
  ]

  return (
    <div className='flex flex-col gap-4'>
      <div className='w-full sm:max-w-xs'>
        <NativeSelect
          value={String(status)}
          onChange={(e) => {
            setStatus(Number(e.target.value))
            setPage(1)
          }}
        >
          {REVIEW_STATUS_FILTERS.map((s) => (
            <NativeSelectOption key={s} value={String(s)}>
              {t(CONTRIBUTION_STATUS_CONFIG[s]?.labelKey ?? 'Unknown')}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      </div>

      <StaticDataTable<Contribution>
        columns={columns}
        data={items}
        getRowKey={(row) => row.id}
        emptyContent={
          listQuery.isLoading ? t('Loading...') : t('Nothing to review')
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

      <ContributionReviewSheet
        contributionId={reviewId}
        onOpenChange={(open) => {
          if (!open) setReviewId(null)
        }}
        onReviewed={() => void listQuery.refetch()}
      />
    </div>
  )
}

function ContributorsList() {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [shareTarget, setShareTarget] = useState<Contributor | null>(null)

  const listQuery = useQuery({
    queryKey: ['contributors', page],
    queryFn: async () =>
      getContributors({ p: page, page_size: CONTRIBUTION_PAGE_SIZE }),
  })

  const items = listQuery.data?.data?.items ?? []
  const total = listQuery.data?.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / CONTRIBUTION_PAGE_SIZE))

  const columns = [
    {
      id: 'username',
      header: t('User'),
      cell: (row: Contributor) => row.display_name || row.username,
    },
    {
      id: 'approved_count',
      header: t('Approved'),
      cell: (row: Contributor) => row.approved_count,
    },
    {
      id: 'pending_count',
      header: t('Pending Review'),
      cell: (row: Contributor) => row.pending_count,
    },
    {
      id: 'share_percent',
      header: t('Share'),
      cell: (row: Contributor) =>
        row.share_percent != null ? `${row.share_percent}%` : t('Default'),
    },
    {
      id: 'contribution_quota',
      header: t('Available Balance'),
      cell: (row: Contributor) =>
        formatQuotaWithCurrency(row.contribution_quota),
    },
    {
      id: 'actions',
      header: t('Actions'),
      className: 'text-right',
      cellClassName: 'text-right',
      cell: (row: Contributor) => (
        <Button variant='outline' size='sm' onClick={() => setShareTarget(row)}>
          {t('Set Share')}
        </Button>
      ),
    },
  ]

  return (
    <div className='flex flex-col gap-4'>
      <StaticDataTable<Contributor>
        columns={columns}
        data={items}
        getRowKey={(row) => row.user_id}
        emptyContent={
          listQuery.isLoading ? t('Loading...') : t('No contributors yet')
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

      <ContributorShareDialog
        contributor={shareTarget}
        onOpenChange={(open) => {
          if (!open) setShareTarget(null)
        }}
        onSaved={() => void listQuery.refetch()}
      />
    </div>
  )
}

export function ContributionReviewPage() {
  const { t } = useTranslation()

  const statsQuery = useQuery({
    queryKey: ['contribution-stats'],
    queryFn: async () => getContributionStats(),
  })

  const stats = statsQuery.data?.data ?? {}
  const statOf = (s: number) => stats[String(s)] ?? 0

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Contribution Review')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='flex flex-col gap-4'>
          <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
            <StatCard
              label={t('Pending Review')}
              value={statOf(CONTRIBUTION_STATUS.PENDING)}
            />
            <StatCard
              label={t('Approved')}
              value={statOf(CONTRIBUTION_STATUS.APPROVED)}
            />
            <StatCard
              label={t('Partially Approved')}
              value={statOf(CONTRIBUTION_STATUS.PARTIALLY_APPROVED)}
            />
            <StatCard
              label={t('Rejected')}
              value={statOf(CONTRIBUTION_STATUS.REJECTED)}
            />
          </div>

          <Tabs defaultValue='queue'>
            <TabsList>
              <TabsTrigger value='queue'>{t('Review Queue')}</TabsTrigger>
              <TabsTrigger value='contributors'>
                {t('Contributors')}
              </TabsTrigger>
            </TabsList>
            <TabsContent value='queue'>
              <ReviewQueue />
            </TabsContent>
            <TabsContent value='contributors'>
              <ContributorsList />
            </TabsContent>
          </Tabs>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
