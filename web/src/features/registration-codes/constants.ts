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

import type { StatusBadgeProps } from '@/components/status-badge'

// ============================================================================
// Registration Code Status Configuration
// ============================================================================

export const REGISTRATION_CODE_STATUS = {
  ENABLED: 1,
  DISABLED: 2,
} as const

export const REGISTRATION_CODE_STATUS_VALUES = Object.values(
  REGISTRATION_CODE_STATUS
).map((value) => String(value)) as `${number}`[]

// labelKey values are i18n keys; use t(config.labelKey) in components
export const REGISTRATION_CODE_STATUSES: Record<
  number,
  Pick<StatusBadgeProps, 'variant'> & {
    labelKey: string
    value: number
  }
> = {
  [REGISTRATION_CODE_STATUS.ENABLED]: {
    labelKey: 'Enabled',
    variant: 'success',
    value: REGISTRATION_CODE_STATUS.ENABLED,
  },
  [REGISTRATION_CODE_STATUS.DISABLED]: {
    labelKey: 'Disabled',
    variant: 'neutral',
    value: REGISTRATION_CODE_STATUS.DISABLED,
  },
} as const

export const REGISTRATION_CODE_FILTER_VALUES = [
  String(REGISTRATION_CODE_STATUS.ENABLED),
  String(REGISTRATION_CODE_STATUS.DISABLED),
] as const

export function getRegistrationCodeStatusOptions(t: TFunction) {
  return Object.values(REGISTRATION_CODE_STATUSES).map((config) => ({
    label: t(config.labelKey),
    value: String(config.value),
  }))
}

// ============================================================================
// Charset Options
// ============================================================================

export const REGISTRATION_CODE_CHARSETS: Record<
  'digits' | 'uppercase' | 'mixed',
  { labelKey: string; value: 'digits' | 'uppercase' | 'mixed' }
> = {
  digits: { labelKey: 'Digits', value: 'digits' },
  uppercase: { labelKey: 'Uppercase letters', value: 'uppercase' },
  mixed: { labelKey: 'Mixed case', value: 'mixed' },
}

export function getRegistrationCodeCharsetOptions(t: TFunction) {
  return Object.values(REGISTRATION_CODE_CHARSETS).map((config) => ({
    label: t(config.labelKey),
    value: config.value,
  }))
}

// ============================================================================
// Validation Constants
// ============================================================================

export const REGISTRATION_CODE_VALIDATION = {
  NAME_MIN_LENGTH: 1,
  NAME_MAX_LENGTH: 64,
  LENGTH_MIN: 4,
  LENGTH_MAX: 32,
  COUNT_MIN: 1,
  COUNT_MAX: 500,
  PREFIX_MAX_LENGTH: 16,
  SUFFIX_MAX_LENGTH: 16,
} as const

// ============================================================================
// Error Messages (i18n keys)
// ============================================================================

export const ERROR_MESSAGES = {
  UNEXPECTED: 'An unexpected error occurred',
  LOAD_FAILED: 'Failed to load registration codes',
  SEARCH_FAILED: 'Failed to search registration codes',
  CREATE_FAILED: 'Failed to create registration code',
  UPDATE_FAILED: 'Failed to update registration code',
  DELETE_FAILED: 'Failed to delete registration code',
  DELETE_INVALID_FAILED: 'Failed to delete invalid registration codes',
  STATUS_UPDATE_FAILED: 'Failed to update registration code status',
  NAME_LENGTH_INVALID:
    'Name must be between {{min}} and {{max}} characters',
  LENGTH_INVALID: 'Length must be between {{min}} and {{max}}',
  COUNT_INVALID: 'Count must be between {{min}} and {{max}}',
  EXPIRED_TIME_INVALID: 'Expired time cannot be earlier than current time',
} as const

/** For form schema only: returns translated messages with interpolation. */
export function getRegistrationCodeFormErrorMessages(t: TFunction) {
  return {
    NAME_LENGTH_INVALID: t(ERROR_MESSAGES.NAME_LENGTH_INVALID, {
      min: REGISTRATION_CODE_VALIDATION.NAME_MIN_LENGTH,
      max: REGISTRATION_CODE_VALIDATION.NAME_MAX_LENGTH,
    }),
    LENGTH_INVALID: t(ERROR_MESSAGES.LENGTH_INVALID, {
      min: REGISTRATION_CODE_VALIDATION.LENGTH_MIN,
      max: REGISTRATION_CODE_VALIDATION.LENGTH_MAX,
    }),
    COUNT_INVALID: t(ERROR_MESSAGES.COUNT_INVALID, {
      min: REGISTRATION_CODE_VALIDATION.COUNT_MIN,
      max: REGISTRATION_CODE_VALIDATION.COUNT_MAX,
    }),
    EXPIRED_TIME_INVALID: t(ERROR_MESSAGES.EXPIRED_TIME_INVALID),
  } as const
}

// ============================================================================
// Success Messages (i18n keys)
// ============================================================================

export const SUCCESS_MESSAGES = {
  REGISTRATION_CODE_CREATED: 'Registration code created successfully',
  REGISTRATION_CODE_UPDATED: 'Registration code updated successfully',
  REGISTRATION_CODE_DELETED: 'Registration code deleted successfully',
  REGISTRATION_CODE_ENABLED: 'Registration code enabled successfully',
  REGISTRATION_CODE_DISABLED: 'Registration code disabled successfully',
  COPY_SUCCESS: 'Copied to clipboard',
} as const
