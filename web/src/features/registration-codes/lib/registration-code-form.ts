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
import type { TFunction } from 'i18next'
import { z } from 'zod'

import {
  REGISTRATION_CODE_VALIDATION,
  getRegistrationCodeFormErrorMessages,
} from '../constants'
import type { RegistrationCodeFormData, RegistrationCode } from '../types'

// ============================================================================
// Form Schema (use getRegistrationCodeFormSchema(t) in components for i18n)
// ============================================================================

export function getRegistrationCodeFormSchema(t: TFunction) {
  const msg = getRegistrationCodeFormErrorMessages(t)
  return z
    .object({
      name: z
        .string()
        .min(REGISTRATION_CODE_VALIDATION.NAME_MIN_LENGTH, msg.NAME_LENGTH_INVALID)
        .max(REGISTRATION_CODE_VALIDATION.NAME_MAX_LENGTH, msg.NAME_LENGTH_INVALID),
      prefix: z.string().max(REGISTRATION_CODE_VALIDATION.PREFIX_MAX_LENGTH),
      suffix: z.string().max(REGISTRATION_CODE_VALIDATION.SUFFIX_MAX_LENGTH),
      max_uses: z.number().min(0, t('Max uses must not be negative')),
      expired_time: z.date().optional(),
      // Random batch generation fields
      charset: z.enum(['digits', 'uppercase', 'mixed']).optional(),
      length: z
        .number()
        .min(REGISTRATION_CODE_VALIDATION.LENGTH_MIN, msg.LENGTH_INVALID)
        .max(REGISTRATION_CODE_VALIDATION.LENGTH_MAX, msg.LENGTH_INVALID)
        .optional(),
      count: z
        .number()
        .min(REGISTRATION_CODE_VALIDATION.COUNT_MIN, msg.COUNT_INVALID)
        .max(REGISTRATION_CODE_VALIDATION.COUNT_MAX, msg.COUNT_INVALID)
        .optional(),
      exclude_confusable: z.boolean().optional(),
      // Manual entry field (one code per line)
      manual_codes: z.string().optional(),
    })
    .refine(
      (data) =>
        Boolean(data.manual_codes?.trim()) ||
        (data.charset !== undefined &&
          data.length !== undefined &&
          data.count !== undefined),
      {
        message: t(
          'Enter at least one code to add manually, or a charset/length/count to generate'
        ),
        path: ['manual_codes'],
      }
    )
}

export type RegistrationCodeFormValues = z.infer<
  ReturnType<typeof getRegistrationCodeFormSchema>
>

// ============================================================================
// Form Defaults
// ============================================================================

export const REGISTRATION_CODE_FORM_DEFAULT_VALUES: RegistrationCodeFormValues = {
  name: '',
  prefix: '',
  suffix: '',
  max_uses: 1,
  expired_time: undefined,
  charset: 'mixed',
  length: 8,
  count: 1,
  exclude_confusable: false,
  manual_codes: '',
}

// ============================================================================
// Form Data Transformation
// ============================================================================

/**
 * Convert manual entry textarea (one per line) to an array.
 * Trims each line; empty lines are dropped server-side too.
 */
export function parseManualCodes(raw: string): string[] {
  return raw
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
}

/**
 * Transform form data to API payload.
 */
export function transformFormDataToPayload(
  data: RegistrationCodeFormValues
): RegistrationCodeFormData {
  const manualCodes = parseManualCodes(data.manual_codes ?? '')
  if (manualCodes.length > 0) {
    return {
      name: data.name,
      prefix: data.prefix,
      suffix: data.suffix,
      max_uses: data.max_uses,
      expired_time: data.expired_time
        ? Math.floor(data.expired_time.getTime() / 1000)
        : 0,
      manual_codes: manualCodes,
    }
  }
  return {
    name: data.name,
    prefix: data.prefix,
    suffix: data.suffix,
    max_uses: data.max_uses,
    expired_time: data.expired_time
      ? Math.floor(data.expired_time.getTime() / 1000)
      : 0,
    charset: data.charset ?? 'mixed',
    length: data.length ?? 8,
    count: data.count ?? 1,
    exclude_confusable: data.exclude_confusable ?? false,
  }
}

/**
 * Transform registration code data to form defaults (for editing).
 */
export function transformRegistrationCodeToFormDefaults(
  code: RegistrationCode
): RegistrationCodeFormValues {
  return {
    name: code.name,
    prefix: code.prefix,
    suffix: code.suffix,
    max_uses: code.max_uses,
    expired_time:
      code.expired_time > 0 ? new Date(code.expired_time * 1000) : undefined,
    charset: 'mixed',
    length: 8,
    count: 1,
    exclude_confusable: false,
    manual_codes: '',
  }
}