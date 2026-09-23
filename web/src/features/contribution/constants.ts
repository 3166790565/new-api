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
import type { StatusVariant } from '@/components/status-badge'

// Contribution status codes (mirror model/contribution.go)
export const CONTRIBUTION_STATUS = {
  PENDING: 0,
  APPROVED: 1,
  PARTIALLY_APPROVED: 2,
  REJECTED: 3,
  WITHDRAWN: 4,
  SUPERSEDED: 5,
} as const

// Per-model review status codes
export const CONTRIBUTION_MODEL_STATUS = {
  PENDING: 0,
  APPROVED: 1,
  REJECTED: 2,
} as const

type StatusConfig = {
  labelKey: string
  variant: StatusVariant
}

// labelKey holds the i18n key (English source string); render via t(config.labelKey).
export const CONTRIBUTION_STATUS_CONFIG: Record<number, StatusConfig> = {
  [CONTRIBUTION_STATUS.PENDING]: {
    labelKey: 'Pending Review',
    variant: 'warning',
  },
  [CONTRIBUTION_STATUS.APPROVED]: { labelKey: 'Approved', variant: 'success' },
  [CONTRIBUTION_STATUS.PARTIALLY_APPROVED]: {
    labelKey: 'Partially Approved',
    variant: 'info',
  },
  [CONTRIBUTION_STATUS.REJECTED]: { labelKey: 'Rejected', variant: 'danger' },
  [CONTRIBUTION_STATUS.WITHDRAWN]: {
    labelKey: 'Withdrawn',
    variant: 'neutral',
  },
  [CONTRIBUTION_STATUS.SUPERSEDED]: {
    labelKey: 'Superseded',
    variant: 'neutral',
  },
}

export const CONTRIBUTION_MODEL_STATUS_CONFIG: Record<number, StatusConfig> = {
  [CONTRIBUTION_MODEL_STATUS.PENDING]: {
    labelKey: 'Pending Review',
    variant: 'warning',
  },
  [CONTRIBUTION_MODEL_STATUS.APPROVED]: {
    labelKey: 'Approved',
    variant: 'success',
  },
  [CONTRIBUTION_MODEL_STATUS.REJECTED]: {
    labelKey: 'Rejected',
    variant: 'danger',
  },
}

// Status filter options for the admin review queue.
export const REVIEW_STATUS_FILTERS: number[] = [
  CONTRIBUTION_STATUS.PENDING,
  CONTRIBUTION_STATUS.APPROVED,
  CONTRIBUTION_STATUS.PARTIALLY_APPROVED,
  CONTRIBUTION_STATUS.REJECTED,
  CONTRIBUTION_STATUS.WITHDRAWN,
]

export const CONTRIBUTION_PAGE_SIZE = 10

// Contributed channels are restricted to the three providers whose upstream
// /models fetch and per-model test are supported by the wizard. Ids mirror
// constant/channel.go (OpenAI=1, Anthropic=14, Gemini=24).
export const CONTRIBUTION_CHANNEL_TYPE_OPTIONS: { id: number; name: string }[] =
  [
    { id: 1, name: 'OpenAI' },
    { id: 14, name: 'Anthropic' },
    { id: 24, name: 'Gemini' },
  ]
