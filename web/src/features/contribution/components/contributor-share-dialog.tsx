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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { handleServerError } from '@/lib/handle-server-error'

import { setContributorSharePercent } from '../api'
import type { Contributor } from '../types'

type ContributorShareDialogProps = {
  contributor: Contributor | null
  onOpenChange: (open: boolean) => void
  onSaved: () => void
}

export function ContributorShareDialog(props: ContributorShareDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [value, setValue] = useState('')

  useEffect(() => {
    if (props.contributor) {
      setValue(
        props.contributor.share_percent != null
          ? String(props.contributor.share_percent)
          : ''
      )
    }
  }, [props.contributor])

  const mutation = useMutation({
    mutationFn: async () => {
      const contributor = props.contributor
      if (!contributor) throw new Error('No contributor selected')
      return setContributorSharePercent({
        user_id: contributor.user_id,
        share_percent: value.trim() === '' ? null : Number(value),
      })
    },
    onSuccess: (result) => {
      if (!result.success) {
        handleServerError(result)
        return
      }
      toast.success(t('Share percent updated'))
      void queryClient.invalidateQueries({ queryKey: ['contributors'] })
      props.onSaved()
      props.onOpenChange(false)
    },
    onError: (error) => handleServerError(error),
  })

  return (
    <ConfirmDialog
      open={props.contributor !== null}
      onOpenChange={props.onOpenChange}
      title={t('Set Share Percent')}
      desc={t(
        'Set the per-request earning share for this contributor. Leave empty to use the system default.'
      )}
      isLoading={mutation.isPending}
      handleConfirm={() => mutation.mutate()}
      confirmText={t('Save')}
    >
      <div className='flex flex-col gap-2'>
        <Label htmlFor='contributor-share'>{t('Share Percent (%)')}</Label>
        <Input
          id='contributor-share'
          type='number'
          min='0'
          max='100'
          value={value}
          placeholder={t('Leave empty to use the default')}
          onChange={(e) => setValue(e.target.value)}
        />
      </div>
    </ConfirmDialog>
  )
}
