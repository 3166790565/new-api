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
import { type FormEvent, useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DateTimePicker } from '@/components/datetime-picker'
import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
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
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { handleServerError } from '@/lib/handle-server-error'
import { addTimeToDate } from '@/lib/time'

import {
  createRegistrationCodes,
  updateRegistrationCode,
  getRegistrationCode,
} from '../api'
import { SUCCESS_MESSAGES } from '../constants'
import {
  getRegistrationCodeFormSchema,
  type RegistrationCodeFormValues,
  REGISTRATION_CODE_FORM_DEFAULT_VALUES,
  transformFormDataToPayload,
  transformRegistrationCodeToFormDefaults,
} from '../lib'
import type {
  RegistrationCode,
  ManualCodesResult,
} from '../types'
import {
  RegistrationCodesExportDialog,
  type RegistrationCodesExportData,
} from './registration-codes-export-dialog'
import { useRegistrationCodes } from './registration-codes-provider'

type RegistrationCodesMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: RegistrationCode
}

export function RegistrationCodesMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: RegistrationCodesMutateDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = !!currentRow
  const codeId = currentRow?.id
  const { triggerRefresh } = useRegistrationCodes()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [createdCodes, setCreatedCodes] =
    useState<RegistrationCodesExportData | null>(null)
  const [codeLoadState, setCodeLoadState] = useState<
    'idle' | 'loading' | 'ready' | 'error'
  >('idle')
  const [loadedCode, setLoadedCode] = useState<RegistrationCode | null>(null)
  const [createMode, setCreateMode] = useState<'generate' | 'manual'>('generate')

  const form = useForm<RegistrationCodeFormValues>({
    resolver: zodResolver(getRegistrationCodeFormSchema(t)),
    defaultValues: REGISTRATION_CODE_FORM_DEFAULT_VALUES,
  })

  // Load existing data when updating
  useEffect(() => {
    if (!open) {
      setCodeLoadState('idle')
      setLoadedCode(null)
      return
    }

    if (!isUpdate || codeId === undefined) {
      form.reset(REGISTRATION_CODE_FORM_DEFAULT_VALUES)
      setCodeLoadState('ready')
      setLoadedCode(null)
      return
    }

    let ignoreResult = false

    form.reset(REGISTRATION_CODE_FORM_DEFAULT_VALUES)
    setCodeLoadState('loading')
    setLoadedCode(null)

    void getRegistrationCode(codeId)
      .then((result) => {
        if (ignoreResult) return

        if (!result.success || !result.data || result.data.id !== codeId) {
          setCodeLoadState('error')
          handleServerError(result, t('Failed to load'))
          return
        }

        form.reset(transformRegistrationCodeToFormDefaults(result.data))
        setLoadedCode(result.data)
        setCodeLoadState('ready')
      })
      .catch((error: unknown) => {
        if (ignoreResult) return

        setCodeLoadState('error')
        handleServerError(error)
      })

    return () => {
      ignoreResult = true
    }
  }, [open, isUpdate, codeId, form, t])

  const isUpdateReady =
    !isUpdate || (codeLoadState === 'ready' && loadedCode?.id === codeId)
  const isLoadingCode = codeLoadState === 'loading'

  const onSubmit = async (data: RegistrationCodeFormValues) => {
    if (isUpdate && (!currentRow || !loadedCode || !isUpdateReady)) {
      return
    }

    setIsSubmitting(true)
    try {
      const basePayload = transformFormDataToPayload(data)

      if (isUpdate && currentRow && loadedCode) {
        const result = await updateRegistrationCode({
          ...basePayload,
          id: currentRow.id,
        })
        if (result.success) {
          toast.success(t(SUCCESS_MESSAGES.REGISTRATION_CODE_UPDATED))
          onOpenChange(false)
          triggerRefresh()
        } else {
          handleServerError(result)
        }
      } else {
        // Create mode
        const result = await createRegistrationCodes(basePayload)
        if (result.success) {
          const isManual = Array.isArray(result.data) === false
          if (isManual) {
            const manual = result.data as ManualCodesResult
            toast.success(
              t('Created {{created}} registration codes', {
                created: manual.created,
              })
            )
            onOpenChange(false)
            triggerRefresh()
            return
          }
          const codes = (result.data as string[]) ?? []
          const count = codes.length
          toast.success(
            count > 1
              ? t('Successfully created {{count}} registration codes', {
                  count,
                })
              : t(SUCCESS_MESSAGES.REGISTRATION_CODE_CREATED)
          )
          if (codes.length > 0) {
            const fullCodes = codes.map(
              (code) => `${data.prefix}${code}${data.suffix}`.toUpperCase()
            )
            setCreatedCodes({
              codes: fullCodes,
              name: basePayload.name,
            })
          }
          onOpenChange(false)
          triggerRefresh()
        } else {
          handleServerError(result)
        }
      }
    } catch (error) {
      handleServerError(error)
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    void form.handleSubmit(onSubmit)(event)
  }

  const handleSetExpiry = (months: number, days: number, hours: number) => {
    const newDate = addTimeToDate(months, days, hours)
    form.setValue('expired_time', newDate)
  }

  let submitButtonLabel = t('Save changes')
  if (isLoadingCode) {
    submitButtonLabel = t('Loading...')
  } else if (isSubmitting) {
    submitButtonLabel = t('Saving...')
  }

  return (
    <>
      <Sheet
        open={open}
        onOpenChange={(v) => {
          onOpenChange(v)
          if (!v) {
            form.reset()
          }
        }}
      >
        <SheetContent
          className={sideDrawerContentClassName('sm:max-w-[600px]')}
        >
          <SheetHeader className={sideDrawerHeaderClassName()}>
            <SheetTitle>
              {isUpdate
                ? t('Update Registration Code')
                : t('Create Registration Code')}
            </SheetTitle>
            <SheetDescription>
              {isUpdate
                ? t('Update the registration code by providing necessary info.')
                : t(
                    'Add new registration code(s) by providing necessary info.'
                  )}{' '}
              {t('Click save when you&apos;re done.')}
            </SheetDescription>
          </SheetHeader>
          <Form {...form}>
            <form
              id='registration-code-form'
              onSubmit={handleSubmit}
              className={sideDrawerFormClassName()}
              aria-busy={isLoadingCode}
            >
              <fieldset
                disabled={!isUpdateReady || isSubmitting}
                className='contents'
              >
                <SideDrawerSection>
                  <FormField
                    control={form.control}
                    name='name'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Name')}</FormLabel>
                        <FormControl>
                          <Input {...field} placeholder={t('Enter a name')} />
                        </FormControl>
                        <FormDescription>
                          {t('Name for this registration code (1-64 characters)')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  {!isUpdate && (
                    <FormItem>
                      <FormLabel>{t('Creation method')}</FormLabel>
                      <FormControl>
                        <RadioGroup
                          value={createMode}
                          onValueChange={(value) => {
                            const mode =
                              value === 'manual' ? 'manual' : 'generate'
                            setCreateMode(mode)
                            form.setValue('manual_codes', '')
                          }}
                          className='flex flex-wrap gap-x-5 gap-y-2'
                        >
                          <div className='flex items-center gap-2'>
                            <RadioGroupItem
                              value='generate'
                              id='rc-mode-generate'
                            />
                            <Label htmlFor='rc-mode-generate'>
                              {t('Random batch generation')}
                            </Label>
                          </div>
                          <div className='flex items-center gap-2'>
                            <RadioGroupItem
                              value='manual'
                              id='rc-mode-manual'
                            />
                            <Label htmlFor='rc-mode-manual'>
                              {t('Manual code entry (one per line)')}
                            </Label>
                          </div>
                        </RadioGroup>
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}

                  {!isUpdate && createMode === 'manual' && (
                    <FormField
                      control={form.control}
                      name='manual_codes'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Manual codes')}</FormLabel>
                          <FormControl>
                            <Textarea
                              rows={6}
                              placeholder={t('One registration code per line')}
                              {...field}
                            />
                          </FormControl>
                          <FormDescription>
                            {t('Duplicates and existing codes are skipped')}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  )}

                  {!isUpdate && createMode === 'generate' && (
                    <>
                      <FormField
                        control={form.control}
                        name='charset'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Charset')}</FormLabel>
                            <FormControl>
                              <Select
                                value={field.value}
                                onValueChange={field.onChange}
                              >
                                <SelectTrigger>
                                  <SelectValue
                                    placeholder={t('Select charset')}
                                  />
                                </SelectTrigger>
                                <SelectContent>
                                  <SelectItem value='digits'>
                                    {t('Digits')}
                                  </SelectItem>
                                  <SelectItem value='uppercase'>
                                    {t('Uppercase letters')}
                                  </SelectItem>
                                  <SelectItem value='mixed'>
                                    {t('Mixed case')}
                                  </SelectItem>
                                </SelectContent>
                              </Select>
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name='exclude_confusable'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Exclude confusable characters')}</FormLabel>
                            <FormControl>
                              <Switch
                                checked={field.value}
                                onCheckedChange={field.onChange}
                              />
                            </FormControl>
                            <FormDescription>
                              {t('Exclude 0 O 1 l I')}
                            </FormDescription>
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name='length'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Length')}</FormLabel>
                            <FormControl>
                              <Input
                                {...field}
                                type='number'
                                min={4}
                                max={32}
                                onChange={(e) =>
                                  field.onChange(
                                    Number.parseInt(e.target.value, 10) || 8
                                  )
                                }
                              />
                            </FormControl>
                            <FormDescription>
                              {t('Between 4 and 32 characters')}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name='count'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Quantity')}</FormLabel>
                            <FormControl>
                              <Input
                                {...field}
                                type='number'
                                min={1}
                                max={500}
                                onChange={(e) =>
                                  field.onChange(
                                    Number.parseInt(e.target.value, 10) || 1
                                  )
                                }
                              />
                            </FormControl>
                            <FormDescription>
                              {t('Create multiple registration codes at once (1-500)')}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </>
                  )}

                  <FormField
                    control={form.control}
                    name='prefix'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Prefix')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            placeholder={t('Optional prefix')}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='suffix'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Suffix')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            placeholder={t('Optional suffix')}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='max_uses'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Max uses per code')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            type='number'
                            min={0}
                            onChange={(e) =>
                              field.onChange(
                                Number.parseInt(e.target.value, 10) || 0
                              )
                            }
                          />
                        </FormControl>
                        <FormDescription>
                          {t('0 means unlimited')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='expired_time'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Expiration Time')}</FormLabel>
                        <div className='flex flex-col gap-2'>
                          <FormControl>
                            <DateTimePicker
                              value={field.value}
                              onChange={field.onChange}
                              placeholder={t('Never expires')}
                            />
                          </FormControl>
                          <div className='grid grid-cols-4 gap-1.5 sm:flex sm:gap-2'>
                            <Button
                              type='button'
                              variant='outline'
                              size='sm'
                              onClick={() => handleSetExpiry(0, 0, 0)}
                            >
                              {t('Never')}
                            </Button>
                            <Button
                              type='button'
                              variant='outline'
                              size='sm'
                              onClick={() => handleSetExpiry(1, 0, 0)}
                            >
                              {t('1M')}
                            </Button>
                            <Button
                              type='button'
                              variant='outline'
                              size='sm'
                              onClick={() => handleSetExpiry(0, 7, 0)}
                            >
                              {t('1W')}
                            </Button>
                            <Button
                              type='button'
                              variant='outline'
                              size='sm'
                              onClick={() => handleSetExpiry(0, 1, 0)}
                            >
                              {t('1 Day')}
                            </Button>
                          </div>
                        </div>
                        <FormDescription>
                          {t('Leave empty for never expires')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </SideDrawerSection>
              </fieldset>
            </form>
          </Form>
          <SheetFooter className={sideDrawerFooterClassName()}>
            <SheetClose render={<Button variant='outline' />}>
              {t('Close')}
            </SheetClose>
            <Button
              form='registration-code-form'
              type='submit'
              disabled={isSubmitting || !isUpdateReady}
            >
              {submitButtonLabel}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
      {createdCodes && (
        <RegistrationCodesExportDialog
          data={createdCodes}
          onClose={() => setCreatedCodes(null)}
        />
      )}
    </>
  )
}
