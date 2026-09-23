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
  ApiResponse,
  Contribution,
  ContributionDetail,
  Contributor,
  EarningsResponse,
  FetchContributionModelsPayload,
  PageEnvelope,
  ReviewContributionPayload,
  SetSharePercentPayload,
  SubmitContributionPayload,
  TestContributionModelPayload,
  TestContributionModelResult,
} from './types'

interface PageParams {
  p?: number
  page_size?: number
}

function pageQuery(params: PageParams = {}): string {
  const { p = 1, page_size = 10 } = params
  const q = new URLSearchParams()
  q.set('p', String(p))
  q.set('page_size', String(page_size))
  return q.toString()
}

// ============================================================================
// User endpoints
// ============================================================================

export async function submitContribution(
  data: SubmitContributionPayload
): Promise<ApiResponse<{ id: number }>> {
  const res = await api.post('/api/contribution/', data)
  return res.data
}

// Pull the upstream model list for the connection details entered in the
// wizard, before any channel is submitted.
export async function fetchContributionModels(
  data: FetchContributionModelsPayload
): Promise<ApiResponse<{ data: string[] }>> {
  const res = await api.post('/api/contribution/fetch-models', data)
  return res.data
}

// Run a single live upstream test for one model. Returns the admin-style
// {success, message, time} shape at the top level of the response.
export async function testContributionModel(
  data: TestContributionModelPayload
): Promise<TestContributionModelResult> {
  const res = await api.post('/api/contribution/test-model', data)
  return res.data
}

export async function getSelfContributions(
  params: PageParams = {}
): Promise<ApiResponse<PageEnvelope<Contribution>>> {
  const res = await api.get(`/api/contribution/self?${pageQuery(params)}`)
  return res.data
}

export async function getSelfContribution(
  id: number
): Promise<ApiResponse<ContributionDetail>> {
  const res = await api.get(`/api/contribution/detail/${id}`)
  return res.data
}

export async function updateSelfContribution(
  id: number,
  data: SubmitContributionPayload
): Promise<ApiResponse> {
  const res = await api.put(`/api/contribution/detail/${id}`, data)
  return res.data
}

export async function withdrawSelfContribution(
  id: number
): Promise<ApiResponse> {
  const res = await api.delete(`/api/contribution/detail/${id}`)
  return res.data
}

export async function getSelfContributionEarnings(
  params: PageParams = {}
): Promise<ApiResponse<EarningsResponse>> {
  const res = await api.get(`/api/contribution/earnings?${pageQuery(params)}`)
  return res.data
}

// Effective share percent for the current user: their per-user override when
// set, otherwise the global default.
export async function getSelfContributionShare(): Promise<
  ApiResponse<{ share_percent: number }>
> {
  const res = await api.get('/api/contribution/share')
  return res.data
}

export async function transferContributionEarnings(
  amount: number
): Promise<ApiResponse<{ contribution_quota: number; quota: number }>> {
  const res = await api.post('/api/contribution/transfer', { amount })
  return res.data
}

// ============================================================================
// Admin endpoints
// ============================================================================

export async function getContributionsForReview(
  status: number,
  params: PageParams = {}
): Promise<ApiResponse<PageEnvelope<Contribution>>> {
  const res = await api.get(
    `/api/contribution/admin/?status=${status}&${pageQuery(params)}`
  )
  return res.data
}

export async function getContributionDetail(
  id: number
): Promise<ApiResponse<ContributionDetail>> {
  const res = await api.get(`/api/contribution/admin/${id}`)
  return res.data
}

export async function reviewContribution(
  data: ReviewContributionPayload
): Promise<ApiResponse<{ status?: number; channel_id?: number }>> {
  const res = await api.post('/api/contribution/admin/review', data)
  return res.data
}

export async function getContributors(
  params: PageParams = {}
): Promise<ApiResponse<PageEnvelope<Contributor>>> {
  const res = await api.get(
    `/api/contribution/admin/contributors?${pageQuery(params)}`
  )
  return res.data
}

export async function getContributionStats(): Promise<
  ApiResponse<Record<string, number>>
> {
  const res = await api.get('/api/contribution/admin/stats')
  return res.data
}

export async function setContributorSharePercent(
  data: SetSharePercentPayload
): Promise<ApiResponse> {
  const res = await api.put('/api/contribution/admin/share', data)
  return res.data
}
