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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMemo, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { safeNumberFieldProps } from '../utils/numeric-field'

// __CONTRIBUTION_SECTION_BODY__
const contributionSchema = z.object({
  contribution_setting: z.object({
    enabled: z.boolean(),
    default_share_percent: z.coerce.number().min(0).max(100),
    max_pending_per_user: z.coerce.number().min(0),
  }),
})

type ContributionFormInput = z.input<typeof contributionSchema>
type ContributionFormValues = z.output<typeof contributionSchema>

type FlatContributionDefaults = {
  'contribution_setting.enabled': boolean
  'contribution_setting.default_share_percent': number
  'contribution_setting.max_pending_per_user': number
}

type ContributionSettingsSectionProps = {
  defaultValues: FlatContributionDefaults
}

const buildFormDefaults = (
  defaults: FlatContributionDefaults
): ContributionFormInput => ({
  contribution_setting: {
    enabled: defaults['contribution_setting.enabled'],
    default_share_percent:
      defaults['contribution_setting.default_share_percent'],
    max_pending_per_user: defaults['contribution_setting.max_pending_per_user'],
  },
})

const normalizeDefaults = (
  defaults: FlatContributionDefaults
): FlatContributionDefaults => ({
  'contribution_setting.enabled': defaults['contribution_setting.enabled'],
  'contribution_setting.default_share_percent':
    defaults['contribution_setting.default_share_percent'],
  'contribution_setting.max_pending_per_user':
    defaults['contribution_setting.max_pending_per_user'],
})

const normalizeFormValues = (
  values: ContributionFormValues
): FlatContributionDefaults => ({
  'contribution_setting.enabled': values.contribution_setting.enabled,
  'contribution_setting.default_share_percent':
    values.contribution_setting.default_share_percent,
  'contribution_setting.max_pending_per_user':
    values.contribution_setting.max_pending_per_user,
})
// __CONTRIBUTION_COMPONENT__
export function ContributionSettingsSection({
  defaultValues,
}: ContributionSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const baselineRef = useRef<FlatContributionDefaults>(
    normalizeDefaults(defaultValues)
  )

  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )

  const form = useForm<ContributionFormInput, unknown, ContributionFormValues>({
    resolver: zodResolver(contributionSchema),
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)

  const enabled = form.watch('contribution_setting.enabled')

  const onSubmit = async (values: ContributionFormValues) => {
    const normalized = normalizeFormValues(values)
    const updates = (
      Object.keys(normalized) as Array<keyof FlatContributionDefaults>
    ).filter((key) => normalized[key] !== baselineRef.current[key])

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const key of updates) {
      await updateOption.mutateAsync({
        key,
        value: normalized[key],
      })
    }

    baselineRef.current = normalized
  }
  // __CONTRIBUTION_RETURN__
  return (
    <SettingsSection title={t('Channel Contribution')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />
          <p className='text-muted-foreground text-sm'>
            {t(
              'Let users submit their own upstream channels for review. Approved channels become routable and the contributor earns a share of the settled charge when other users route through them.'
            )}
          </p>

          <FormField
            control={form.control}
            name='contribution_setting.enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable channel contribution')}</FormLabel>
                  <FormDescription>
                    {t(
                      'When disabled, users cannot submit channels and the contribution pages are hidden from routing.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='contribution_setting.default_share_percent'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Default share percent')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      max={100}
                      step={1}
                      {...safeNumberFieldProps(field)}
                      disabled={!enabled}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Share of the settled charge paid to a contributor without a per-user override (0-100). 0 disables payouts.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='contribution_setting.max_pending_per_user'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Max pending submissions per user')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      step={1}
                      {...safeNumberFieldProps(field)}
                      disabled={!enabled}
                    />
                  </FormControl>
                  <FormDescription>{t('0 means no limit.')}</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
