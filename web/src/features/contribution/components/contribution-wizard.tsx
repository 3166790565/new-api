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
import {
  CheckCircle2,
  Download,
  Loader2,
  RotateCw,
  Trash2,
  XCircle,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { UpstreamModelSelection } from '@/features/channels/components/upstream-model-selection'
import { handleServerError } from '@/lib/handle-server-error'
import { cn } from '@/lib/utils'

import {
  fetchContributionModels,
  getSelfContribution,
  submitContribution,
  testContributionModel,
  updateSelfContribution,
} from '../api'
import { CONTRIBUTION_CHANNEL_TYPE_OPTIONS } from '../constants'

//__WIZARD_TYPES__

type TestStatus = 'idle' | 'testing' | 'success' | 'failed'

type TestState = {
  status: TestStatus
  time?: number
  error?: string
}

type ContributionWizardProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  contributionId?: number
  onSaved: () => void
}

const STEP_KEYS = [
  {
    titleKey: 'Connection & models',
    descKey: 'Enter connection info and pull models from the upstream',
  },
  {
    titleKey: 'Test models',
    descKey: 'Test each model; delete any that fail, then continue',
  },
  {
    titleKey: 'Submit for review',
    descKey: 'Confirm and submit to an administrator',
  },
]

// Bounded concurrency runner so a "Test all" does not fan out unbounded.
async function runPool<T>(
  items: T[],
  limit: number,
  worker: (item: T) => Promise<void>
) {
  let cursor = 0
  const runners = Array.from({ length: Math.min(limit, items.length) }, () =>
    (async () => {
      while (cursor < items.length) {
        const index = cursor
        cursor += 1
        await worker(items[index])
      }
    })()
  )
  await Promise.all(runners)
}

//__WIZARD_COMPONENT__

export function ContributionWizard(props: ContributionWizardProps) {
  const { t } = useTranslation()
  const isUpdate = props.contributionId !== undefined

  const [step, setStep] = useState(0)
  const [type, setType] = useState<number>(
    CONTRIBUTION_CHANNEL_TYPE_OPTIONS[0].id
  )
  const [name, setName] = useState('')
  const [baseUrl, setBaseUrl] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [group, setGroup] = useState('default')

  const [fetchedModels, setFetchedModels] = useState<string[]>([])
  const [selectedModels, setSelectedModels] = useState<string[]>([])
  const [testStates, setTestStates] = useState<Record<string, TestState>>({})
  const [agreed, setAgreed] = useState(false)

  const [isFetching, setIsFetching] = useState(false)
  const [isTesting, setIsTesting] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [loadState, setLoadState] = useState<
    'idle' | 'loading' | 'ready' | 'error'
  >('idle')

  // Reset everything to a clean state for a new submission.
  const resetAll = () => {
    setStep(0)
    setType(CONTRIBUTION_CHANNEL_TYPE_OPTIONS[0].id)
    setName('')
    setBaseUrl('')
    setApiKey('')
    setGroup('default')
    setFetchedModels([])
    setSelectedModels([])
    setTestStates({})
    setAgreed(false)
  }

  useEffect(() => {
    if (!props.open) {
      setLoadState('idle')
      return
    }
    if (!isUpdate || props.contributionId === undefined) {
      resetAll()
      setLoadState('ready')
      return
    }
    let ignore = false
    resetAll()
    setLoadState('loading')
    void getSelfContribution(props.contributionId)
      .then((result) => {
        if (ignore) return
        if (!result.success || !result.data) {
          setLoadState('error')
          handleServerError(result, t('Failed to load'))
          return
        }
        const detail = result.data
        setType(detail.type)
        setName(detail.name)
        setBaseUrl(detail.base_url ?? '')
        setApiKey('')
        setGroup(detail.group || 'default')
        const models = detail.models.map((m) => m.upstream_model)
        setFetchedModels(models)
        setSelectedModels(models)
        setLoadState('ready')
      })
      .catch((error: unknown) => {
        if (ignore) return
        setLoadState('error')
        handleServerError(error)
      })
    return () => {
      ignore = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [props.open, props.contributionId, isUpdate])

  //__WIZARD_HANDLERS__

  // Shared connection-field validation. Both the model fetch and the model
  // test need a real upstream key, so it is always required here.
  const validateConnection = (): boolean => {
    if (!name.trim()) {
      toast.error(t('Channel name is required'))
      return false
    }
    if (baseUrl.trim().endsWith('/')) {
      toast.error(t('API address must not end with a slash'))
      return false
    }
    if (!apiKey.trim()) {
      toast.error(t('Key is required'))
      return false
    }
    return true
  }

  const handleFetch = async () => {
    if (!validateConnection()) return
    setIsFetching(true)
    try {
      const result = await fetchContributionModels({
        type,
        base_url: baseUrl.trim(),
        key: apiKey.trim(),
      })
      if (!result.success) {
        handleServerError(result)
        return
      }
      const models = result.data?.data ?? []
      setFetchedModels(models)
      if (models.length === 0) {
        toast.warning(t('No models returned by the upstream'))
      } else {
        toast.success(t('Fetched {{count}} models', { count: models.length }))
      }
    } catch (error) {
      handleServerError(error)
    } finally {
      setIsFetching(false)
    }
  }

  const testOne = async (model: string) => {
    setTestStates((prev) => ({ ...prev, [model]: { status: 'testing' } }))
    try {
      const result = await testContributionModel({
        type,
        base_url: baseUrl.trim(),
        key: apiKey.trim(),
        model,
      })
      setTestStates((prev) => ({
        ...prev,
        [model]: result.success
          ? { status: 'success', time: result.time }
          : { status: 'failed', error: result.message, time: result.time },
      }))
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
      setTestStates((prev) => ({
        ...prev,
        [model]: { status: 'failed', error: message },
      }))
    }
  }

  const handleTestAll = async () => {
    const pending = selectedModels.filter(
      (m) => testStates[m]?.status !== 'success'
    )
    if (pending.length === 0) return
    setIsTesting(true)
    try {
      await runPool(pending, 3, testOne)
    } finally {
      setIsTesting(false)
    }
  }

  const deleteModel = (model: string) => {
    setSelectedModels((prev) => prev.filter((m) => m !== model))
    setTestStates((prev) => {
      const next = { ...prev }
      delete next[model]
      return next
    })
  }

  const allPassed =
    selectedModels.length > 0 &&
    selectedModels.every((m) => testStates[m]?.status === 'success')

  const handleNext = () => {
    if (step === 0) {
      if (!validateConnection()) return
      if (selectedModels.length === 0) {
        toast.error(t('Select at least one model'))
        return
      }
    }
    if (step === 1 && !allPassed) {
      toast.error(t('All selected models must pass the test to continue'))
      return
    }
    setStep((s) => Math.min(s + 1, STEP_KEYS.length - 1))
  }

  const handleBack = () => setStep((s) => Math.max(s - 1, 0))

  const handleSubmit = async () => {
    if (!agreed) {
      toast.error(t('Please read and accept the contribution agreement'))
      return
    }
    if (!allPassed) {
      toast.error(t('All selected models must pass the test to continue'))
      return
    }
    setIsSubmitting(true)
    try {
      const payload = {
        type,
        name: name.trim(),
        base_url: baseUrl.trim(),
        key: apiKey.trim(),
        group: group.trim() || 'default',
        priority: 0,
        weight: 0,
        test_model: '',
        models: selectedModels.map((m) => ({
          upstream_model: m,
          public_model: m,
        })),
      }
      const result = isUpdate
        ? await updateSelfContribution(props.contributionId as number, payload)
        : await submitContribution(payload)
      if (result.success) {
        toast.success(
          isUpdate
            ? t('Contribution updated')
            : t('Contribution submitted for review')
        )
        props.onOpenChange(false)
        props.onSaved()
      } else {
        handleServerError(result)
      }
    } catch (error) {
      handleServerError(error)
    } finally {
      setIsSubmitting(false)
    }
  }

  const isReady = !isUpdate || loadState === 'ready'
  const isLastStep = step === STEP_KEYS.length - 1

  //__WIZARD_RETURN__

  return (
    <Sheet
      open={props.open}
      onOpenChange={(v) => {
        props.onOpenChange(v)
        if (!v) resetAll()
      }}
    >
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[680px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isUpdate ? t('Edit Contribution') : t('Contribute a Channel')}
          </SheetTitle>
          <SheetDescription>
            {t('Only the connection info needed for review is collected.')}
          </SheetDescription>
          <ol className='mt-3 grid grid-cols-3 gap-2'>
            {STEP_KEYS.map((s, index) => {
              const isActive = step === index
              const isCompleted = step > index
              return (
                <li
                  key={s.titleKey}
                  className={cn('rounded-lg border p-2', {
                    'border-primary ring-primary/20 ring-1': isActive,
                    'border-primary/40 bg-primary/5': !isActive && isCompleted,
                    'border-muted': !isActive && !isCompleted,
                  })}
                >
                  <div className='flex items-center gap-2'>
                    <span
                      className={cn(
                        'flex size-5 shrink-0 items-center justify-center rounded-md border text-[11px] font-semibold',
                        isActive || isCompleted
                          ? 'border-primary bg-primary text-primary-foreground'
                          : 'border-muted-foreground/40 text-muted-foreground'
                      )}
                    >
                      {index + 1}
                    </span>
                    <span className='truncate text-xs font-medium'>
                      {t(s.titleKey)}
                    </span>
                  </div>
                </li>
              )
            })}
          </ol>
        </SheetHeader>
        <div
          className={sideDrawerFormClassName()}
          aria-busy={loadState === 'loading'}
        >
          {/*STEP_CONTENT*/}
          {step === 0 && (
            <>
              <SideDrawerSection>
                <div className='flex flex-col gap-2'>
                  <Label htmlFor='contrib-type'>{t('Channel Type')}</Label>
                  <NativeSelect
                    id='contrib-type'
                    value={String(type)}
                    disabled={!isReady}
                    onChange={(e) => setType(Number(e.target.value))}
                  >
                    {CONTRIBUTION_CHANNEL_TYPE_OPTIONS.map((option) => (
                      <NativeSelectOption
                        key={option.id}
                        value={String(option.id)}
                      >
                        {option.name}
                      </NativeSelectOption>
                    ))}
                  </NativeSelect>
                </div>
                <div className='flex flex-col gap-2'>
                  <Label htmlFor='contrib-name'>{t('Name')}</Label>
                  <Input
                    id='contrib-name'
                    value={name}
                    disabled={!isReady}
                    placeholder={t('Enter a name')}
                    onChange={(e) => setName(e.target.value)}
                  />
                </div>
                <div className='flex flex-col gap-2'>
                  <Label htmlFor='contrib-base-url'>{t('API Address')}</Label>
                  <Input
                    id='contrib-base-url'
                    value={baseUrl}
                    disabled={!isReady}
                    placeholder={t('Leave empty to use the default')}
                    onChange={(e) => setBaseUrl(e.target.value)}
                  />
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'Use the provider base URL without a model-specific path. It must not end with a slash.'
                    )}
                  </p>
                </div>
                <div className='flex flex-col gap-2'>
                  <Label htmlFor='contrib-key'>{t('Key')}</Label>
                  <Input
                    id='contrib-key'
                    type='password'
                    autoComplete='off'
                    value={apiKey}
                    disabled={!isReady}
                    placeholder={
                      isUpdate
                        ? t('Re-enter the key to update it')
                        : t('Enter the upstream API key')
                    }
                    onChange={(e) => setApiKey(e.target.value)}
                  />
                </div>
                <div className='flex flex-col gap-2'>
                  <Label htmlFor='contrib-group'>{t('Group')}</Label>
                  <Input
                    id='contrib-group'
                    value={group}
                    disabled={!isReady}
                    onChange={(e) => setGroup(e.target.value)}
                  />
                </div>
              </SideDrawerSection>
              <SideDrawerSection>
                <div className='flex items-center justify-between'>
                  <Label>{t('Models')}</Label>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    disabled={!isReady || isFetching}
                    onClick={() => void handleFetch()}
                  >
                    {isFetching ? (
                      <Loader2 className='size-4 animate-spin' aria-hidden />
                    ) : (
                      <Download className='size-4' aria-hidden />
                    )}
                    {t('Fetch Models')}
                  </Button>
                </div>
                {fetchedModels.length > 0 ? (
                  <UpstreamModelSelection
                    models={fetchedModels}
                    selected={selectedModels}
                    onChange={setSelectedModels}
                    existingModels={[]}
                    showChanges={false}
                    summaryText={t('Fetched {{count}} models', {
                      count: fetchedModels.length,
                    })}
                  />
                ) : (
                  <p className='text-muted-foreground text-sm'>
                    {t(
                      'Fetch models from the upstream, then choose which ones to add.'
                    )}
                  </p>
                )}
              </SideDrawerSection>
            </>
          )}
          {/*STEP_CONTENT_2*/}
          {step === 1 && (
            <SideDrawerSection>
              <div className='flex items-center justify-between gap-2'>
                <div>
                  <Label>{t('Test models')}</Label>
                  <p className='text-muted-foreground mt-1 text-xs'>
                    {t(
                      'Each model is tested against the upstream. Delete any that fail, then continue.'
                    )}
                  </p>
                </div>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  disabled={isTesting || selectedModels.length === 0}
                  onClick={() => void handleTestAll()}
                >
                  {isTesting ? (
                    <Loader2 className='size-4 animate-spin' aria-hidden />
                  ) : (
                    <RotateCw className='size-4' aria-hidden />
                  )}
                  {t('Test All')}
                </Button>
              </div>
              <div className='flex flex-col gap-2'>
                {selectedModels.map((model) => {
                  const state = testStates[model] ?? { status: 'idle' }
                  return (
                    <div
                      key={model}
                      className='flex items-center gap-2 rounded-lg border px-3 py-2'
                    >
                      <div className='min-w-0 flex-1'>
                        <p className='truncate font-mono text-sm'>{model}</p>
                        {state.status === 'failed' && state.error && (
                          <p className='text-destructive mt-0.5 truncate text-xs'>
                            {state.error}
                          </p>
                        )}
                      </div>
                      {state.status === 'success' && (
                        <Badge variant='outline' className='gap-1 text-xs'>
                          <CheckCircle2
                            className='size-3.5 text-emerald-500'
                            aria-hidden
                          />
                          {typeof state.time === 'number'
                            ? `${state.time.toFixed(2)}s`
                            : t('Passed')}
                        </Badge>
                      )}
                      {state.status === 'failed' && (
                        <Badge variant='outline' className='gap-1 text-xs'>
                          <XCircle
                            className='text-destructive size-3.5'
                            aria-hidden
                          />
                          {t('Failed')}
                        </Badge>
                      )}
                      {state.status === 'testing' && (
                        <Loader2
                          className='text-muted-foreground size-4 animate-spin'
                          aria-hidden
                        />
                      )}
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon'
                        aria-label={t('Test')}
                        disabled={state.status === 'testing'}
                        onClick={() => void testOne(model)}
                      >
                        <RotateCw className='size-4' aria-hidden />
                      </Button>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon'
                        aria-label={t('Remove')}
                        onClick={() => deleteModel(model)}
                      >
                        <Trash2 className='size-4' aria-hidden />
                      </Button>
                    </div>
                  )
                })}
                {selectedModels.length === 0 && (
                  <p className='text-muted-foreground text-sm'>
                    {t('No models selected. Go back and add some.')}
                  </p>
                )}
              </div>
            </SideDrawerSection>
          )}
          {/*STEP_CONTENT_3*/}
          {step === 2 && (
            <SideDrawerSection>
              <div className='flex flex-col gap-3 text-sm'>
                <div className='flex justify-between gap-2'>
                  <span className='text-muted-foreground'>{t('Name')}</span>
                  <span className='truncate font-medium'>{name}</span>
                </div>
                <div className='flex justify-between gap-2'>
                  <span className='text-muted-foreground'>
                    {t('Channel Type')}
                  </span>
                  <span className='font-medium'>
                    {CONTRIBUTION_CHANNEL_TYPE_OPTIONS.find(
                      (o) => o.id === type
                    )?.name ?? type}
                  </span>
                </div>
                <div className='flex justify-between gap-2'>
                  <span className='text-muted-foreground'>{t('Group')}</span>
                  <span className='font-medium'>{group || 'default'}</span>
                </div>
                <div className='flex justify-between gap-2'>
                  <span className='text-muted-foreground'>{t('Models')}</span>
                  <span className='font-medium'>
                    {t('{{count}} passed', { count: selectedModels.length })}
                  </span>
                </div>
              </div>
              <div className='flex flex-wrap gap-1.5'>
                {selectedModels.map((model) => (
                  <Badge key={model} variant='outline' className='font-mono'>
                    {model}
                  </Badge>
                ))}
              </div>
              <label className='flex cursor-pointer items-start gap-2'>
                <Checkbox
                  className='mt-0.5'
                  checked={agreed}
                  onCheckedChange={(checked) => setAgreed(checked === true)}
                />
                <span className='text-sm'>
                  {t('I have read and accept the contribution agreement')}
                </span>
              </label>
            </SideDrawerSection>
          )}
        </div>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button variant='outline' />}>
            {t('Close')}
          </SheetClose>
          {step > 0 && (
            <Button variant='outline' onClick={handleBack}>
              {t('Back')}
            </Button>
          )}
          {!isLastStep && (
            <Button onClick={handleNext} disabled={!isReady}>
              {t('Next')}
            </Button>
          )}
          {isLastStep && (
            <Button
              onClick={() => void handleSubmit()}
              disabled={!isReady || isSubmitting || !agreed || !allPassed}
            >
              {isSubmitting ? t('Saving...') : t('Submit for review')}
            </Button>
          )}
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
