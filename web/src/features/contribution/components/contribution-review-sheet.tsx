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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Textarea } from '@/components/ui/textarea'
import { CHANNEL_TYPES } from '@/features/channels/constants'
import { handleServerError } from '@/lib/handle-server-error'

import { getContributionDetail, reviewContribution } from '../api'
import { CONTRIBUTION_STATUS } from '../constants'
import type { ReviewDecision } from '../types'
import { ContributionModelStatusBadge } from './contribution-status-badge'

type ModelDecisionState = {
  model_id: number
  approve: boolean
  public_model: string
}

type ContributionReviewSheetProps = {
  contributionId: number | null
  onOpenChange: (open: boolean) => void
  onReviewed: () => void
}

export function ContributionReviewSheet(props: ContributionReviewSheetProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const open = props.contributionId !== null

  const [decisions, setDecisions] = useState<ModelDecisionState[]>([])
  const [sharePercent, setSharePercent] = useState('')
  const [rejectReason, setRejectReason] = useState('')

  const detailQuery = useQuery({
    queryKey: ['contribution-detail', props.contributionId],
    queryFn: async () => getContributionDetail(props.contributionId as number),
    enabled: open,
  })

  const detail = detailQuery.data?.data

  useEffect(() => {
    if (!detail) return
    setDecisions(
      detail.models.map((m) => ({
        model_id: m.id,
        approve: true,
        public_model: m.public_model,
      }))
    )
    setSharePercent(
      detail.share_percent != null ? String(detail.share_percent) : ''
    )
    setRejectReason('')
  }, [detail])

  const reviewMutation = useMutation({
    mutationFn: async (action: 'approve' | 'reject') => {
      const parsedShare =
        sharePercent.trim() === '' ? null : Number(sharePercent)
      const payloadDecisions: ReviewDecision[] = decisions.map((d) => ({
        model_id: d.model_id,
        approve: d.approve,
        public_model: d.public_model.trim(),
        reject_reason: '',
      }))
      return reviewContribution({
        id: props.contributionId as number,
        action,
        share_percent: action === 'approve' ? parsedShare : undefined,
        reject_reason: action === 'reject' ? rejectReason.trim() : undefined,
        decisions: action === 'approve' ? payloadDecisions : undefined,
      })
    },
    onSuccess: (result, action) => {
      if (!result.success) {
        handleServerError(result)
        return
      }
      toast.success(
        action === 'approve'
          ? t('Contribution approved')
          : t('Contribution rejected')
      )
      void queryClient.invalidateQueries({
        queryKey: ['review-contributions'],
      })
      void queryClient.invalidateQueries({ queryKey: ['contribution-stats'] })
      props.onReviewed()
      props.onOpenChange(false)
    },
    onError: (error) => handleServerError(error),
  })

  const setDecision = (id: number, patch: Partial<ModelDecisionState>) => {
    setDecisions((prev) =>
      prev.map((d) => (d.model_id === id ? { ...d, ...patch } : d))
    )
  }

  const isPending = detail?.status === CONTRIBUTION_STATUS.PENDING
  const hasApproved = decisions.some((d) => d.approve)

  return (
    <Sheet
      open={open}
      onOpenChange={(v) => {
        if (!v) props.onOpenChange(false)
      }}
    >
      <SheetContent className='flex w-full flex-col gap-0 sm:max-w-[680px]'>
        <SheetHeader>
          <SheetTitle>{t('Review Contribution')}</SheetTitle>
          <SheetDescription>
            {detail ? `#${detail.id} · ${detail.name}` : t('Loading...')}
          </SheetDescription>
        </SheetHeader>

        <div className='flex flex-col gap-4 overflow-y-auto px-4 py-4'>
          {detail && (
            <>
              <dl className='grid grid-cols-2 gap-x-4 gap-y-2 text-sm'>
                <div>
                  <dt className='text-muted-foreground'>{t('Channel Type')}</dt>
                  <dd>
                    {CHANNEL_TYPES[detail.type as keyof typeof CHANNEL_TYPES] ??
                      detail.type}
                  </dd>
                </div>
                <div>
                  <dt className='text-muted-foreground'>{t('Group')}</dt>
                  <dd>{detail.group}</dd>
                </div>
                <div>
                  <dt className='text-muted-foreground'>{t('Key')}</dt>
                  <dd className='font-mono text-xs'>{detail.key}</dd>
                </div>
                <div>
                  <dt className='text-muted-foreground'>{t('Base URL')}</dt>
                  <dd className='break-all'>{detail.base_url || '-'}</dd>
                </div>
              </dl>

              <div className='flex flex-col gap-2'>
                <Label>{t('Models')}</Label>
                {decisions.map((decision) => {
                  const model = detail.models.find(
                    (m) => m.id === decision.model_id
                  )
                  if (!model) return null
                  return (
                    <div
                      key={decision.model_id}
                      className='flex flex-col gap-2 rounded-lg border p-3'
                    >
                      <div className='flex items-center justify-between gap-2'>
                        <span className='font-mono text-sm'>
                          {model.upstream_model}
                        </span>
                        <ContributionModelStatusBadge status={model.status} />
                      </div>
                      {isPending ? (
                        <>
                          <div className='flex items-center gap-2'>
                            <Checkbox
                              id={`approve-${decision.model_id}`}
                              checked={decision.approve}
                              onCheckedChange={(checked) =>
                                setDecision(decision.model_id, {
                                  approve: checked === true,
                                })
                              }
                            />
                            <Label htmlFor={`approve-${decision.model_id}`}>
                              {t('Approve this model')}
                            </Label>
                          </div>
                          <Input
                            value={decision.public_model}
                            disabled={!decision.approve}
                            placeholder={t('Public model name')}
                            onChange={(e) =>
                              setDecision(decision.model_id, {
                                public_model: e.target.value,
                              })
                            }
                          />
                        </>
                      ) : (
                        <span className='text-muted-foreground text-sm'>
                          {t('Public model')}: {model.public_model}
                        </span>
                      )}
                    </div>
                  )
                })}
              </div>

              {isPending && (
                <>
                  <div className='flex flex-col gap-2'>
                    <Label htmlFor='share-percent'>
                      {t('Share Percent (%)')}
                    </Label>
                    <Input
                      id='share-percent'
                      type='number'
                      min='0'
                      max='100'
                      value={sharePercent}
                      placeholder={t('Leave empty to use the default')}
                      onChange={(e) => setSharePercent(e.target.value)}
                    />
                  </div>
                  <div className='flex flex-col gap-2'>
                    <Label htmlFor='reject-reason'>{t('Reject Reason')}</Label>
                    <Textarea
                      id='reject-reason'
                      value={rejectReason}
                      placeholder={t('Required when rejecting')}
                      onChange={(e) => setRejectReason(e.target.value)}
                    />
                  </div>
                </>
              )}

              {!isPending && detail.reject_reason && (
                <div className='text-sm'>
                  <span className='text-muted-foreground'>
                    {t('Reject Reason')}:{' '}
                  </span>
                  {detail.reject_reason}
                </div>
              )}
            </>
          )}
        </div>

        <SheetFooter>
          <SheetClose render={<Button variant='outline' />}>
            {t('Close')}
          </SheetClose>
          {isPending && (
            <>
              <Button
                variant='destructive'
                disabled={reviewMutation.isPending}
                onClick={() => reviewMutation.mutate('reject')}
              >
                {t('Reject')}
              </Button>
              <Button
                disabled={reviewMutation.isPending || !hasApproved}
                onClick={() => reviewMutation.mutate('approve')}
              >
                {t('Approve')}
              </Button>
            </>
          )}
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
