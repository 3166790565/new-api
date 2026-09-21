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
import { api } from '@/lib/api'

import type {
  RegistrationCode,
  ApiResponse,
  GetRegistrationCodesParams,
  GetRegistrationCodesResponse,
  SearchRegistrationCodesParams,
  RegistrationCodeFormData,
  ManualCodesResult,
} from './types'

// ============================================================================
// Registration Code Management
// ============================================================================

// Get paginated registration codes list
export async function getRegistrationCodes(
  params: GetRegistrationCodesParams = {}
): Promise<GetRegistrationCodesResponse> {
  const { p = 1, page_size = 10 } = params
  const res = await api.get(
    `/api/registration_code/?p=${p}&page_size=${page_size}`
  )
  return res.data
}

// Search registration codes by keyword / status
export async function searchRegistrationCodes(
  params: SearchRegistrationCodesParams
): Promise<GetRegistrationCodesResponse> {
  const { keyword = '', status = '', p = 1, page_size = 10 } = params
  const queryParams = new URLSearchParams()
  queryParams.set('keyword', keyword)
  if (status) queryParams.set('status', status)
  queryParams.set('p', String(p))
  queryParams.set('page_size', String(page_size))
  const res = await api.get(
    `/api/registration_code/search?${queryParams.toString()}`
  )
  return res.data
}

// Get single registration code by ID
export async function getRegistrationCode(
  id: number
): Promise<ApiResponse<RegistrationCode>> {
  const res = await api.get(`/api/registration_code/${id}`)
  return res.data
}

// Create registration code(s): random batch or manual entry
export async function createRegistrationCodes(
  data: RegistrationCodeFormData
): Promise<ApiResponse<string[] | ManualCodesResult>> {
  const res = await api.post('/api/registration_code/', data)
  return res.data
}

// Update registration code
export async function updateRegistrationCode(
  data: RegistrationCodeFormData & { id: number }
): Promise<ApiResponse<RegistrationCode>> {
  const res = await api.put('/api/registration_code/', data)
  return res.data
}

// Update registration code status (enable/disable)
export async function updateRegistrationCodeStatus(
  id: number,
  status: number
): Promise<ApiResponse<RegistrationCode>> {
  const res = await api.put('/api/registration_code/?status_only=true', {
    id,
    status,
  })
  return res.data
}

// Delete a single registration code
export async function deleteRegistrationCode(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/registration_code/${id}`)
  return res.data
}

// Delete invalid registration codes (disabled, expired, exhausted)
export async function deleteInvalidRegistrationCodes(): Promise<
  ApiResponse<number>
> {
  const res = await api.delete('/api/registration_code/invalid')
  return res.data
}

export async function batchDeleteRegistrationCodes(
  ids: number[]
): Promise<ApiResponse<number>> {
  const res = await api.post('/api/registration_code/batch', { ids })
  return res.data
}
