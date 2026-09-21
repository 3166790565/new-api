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
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'

export type RegistrationCodesExportData = {
  codes: string[]
  name: string
}

type RegistrationCodesExportDialogProps = {
  data: RegistrationCodesExportData
  onClose: () => void
}

export function RegistrationCodesExportDialog(
  props: RegistrationCodesExportDialogProps
) {
  const { t } = useTranslation()
  const id = useId()
  const [saveToFile, setSaveToFile] = useState(false)
  const [format, setFormat] = useState<'txt' | 'md'>('txt')
  const [includeName, setIncludeName] = useState(true)

  const handleComplete = () => {
    if (!saveToFile) {
      props.onClose()
      return
    }
    const rows = props.data.codes.map((value) => value.replaceAll(/[\t\r\n]+/g, ' '))
    let content = `${rows.join('\n')}\n`
    if (format === 'md') {
      const headers = includeName ? [t('Name'), t('Code')] : [t('Code')]
      const markdownRows = [headers, headers.map(() => '---')]
      if (includeName) {
        markdownRows.push(
          ...props.data.codes.map((value) => [
            props.data.name,
            value.replaceAll(/[\\`*_[\]|<>]/g, '\\$&'),
          ])
        )
      } else {
        markdownRows.push(
          ...props.data.codes.map((value) => [
            value.replaceAll(/[\\`*_[\]|<>]/g, '\\$&'),
          ])
        )
      }
      content = `${markdownRows
        .map((row, index) => {
          const cells =
            index === 1
              ? row
              : row.map((value) =>
                  value.replaceAll('&', '&amp;').replaceAll(/[\\`*_[\]|<>]/g, '\\$&')
                )
          return `| ${cells.join(' | ')} |`
        })
        .join('\n')}\n`
    }

    const blob = new Blob([content], {
      type:
        format === 'md'
          ? 'text/markdown;charset=utf-8'
          : 'text/plain;charset=utf-8',
    })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `registration-codes-${Date.now()}.${format}`
    document.body.append(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
    props.onClose()
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) props.onClose()
      }}
      title={t('Registration codes created')}
      description={t('Successfully created {{count}} registration codes', {
        count: props.data.codes.length,
      })}
      contentClassName='sm:max-w-md'
      bodyClassName='space-y-5'
      footer={<Button onClick={handleComplete}>{t('Done')}</Button>}
    >
      <div className='flex items-center gap-2'>
        <Checkbox
          id={`${id}-save-file`}
          checked={saveToFile}
          onCheckedChange={setSaveToFile}
        />
        <Label htmlFor={`${id}-save-file`}>{t('Save as a file')}</Label>
      </div>
      {saveToFile && (
        <div className='space-y-4 pl-6'>
          <RadioGroup
            value={format}
            onValueChange={(value) => {
              if (value === 'txt' || value === 'md') {
                setFormat(value)
              }
            }}
            aria-label={t('Save registration codes')}
            className='flex flex-wrap gap-x-5 gap-y-3'
          >
            {[
              { value: 'txt', label: t('Save as TXT') },
              { value: 'md', label: t('Save as Markdown') },
            ].map((option) => (
              <div key={option.value} className='flex items-center gap-2'>
                <RadioGroupItem
                  value={option.value}
                  id={`${id}-${option.value}`}
                />
                <Label htmlFor={`${id}-${option.value}`}>{option.label}</Label>
              </div>
            ))}
          </RadioGroup>
          <div className='flex flex-wrap gap-6'>
            <div className='flex items-center gap-2'>
              <Checkbox
                id={`${id}-name`}
                checked={includeName}
                onCheckedChange={setIncludeName}
              />
              <Label htmlFor={`${id}-name`}>{t('Include name')}</Label>
            </div>
          </div>
        </div>
      )}
    </Dialog>
  )
}