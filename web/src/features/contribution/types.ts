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

// ============================================================================
// Channel Contribution Types (mirror model/contribution.go)
// ============================================================================

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface PageEnvelope<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

export interface ContributionModel {
  id: number
  contribution_id: number
  upstream_model: string
  public_model: string
  status: number // 0 pending, 1 approved, 2 rejected
  reject_reason: string
  channel_id: number
  created_time: number
}

export interface Contribution {
  id: number
  user_id: number
  type: number
  name: string
  base_url: string
  key: string
  group: string
  priority: number
  weight: number
  status: number // 0 pending, 1 approved, 2 partially approved, 3 rejected, 4 withdrawn, 5 superseded
  reject_reason: string
  share_percent: number | null
  source_channel_id: number
  test_model: string | null
  test_time: number
  reviewer_id: number
  reviewed_time: number
  created_time: number
}

export interface ContributionDetail extends Contribution {
  models: ContributionModel[]
}

export interface ContributionEarning {
  id: number
  contributor_user_id: number
  caller_user_id: number
  contribution_id: number
  channel_id: number
  model_name: string
  upstream_model: string
  request_id: string
  caller_quota: number
  share_percent: number
  share_quota: number
  created_time: number
}

export interface EarningsResponse {
  page: PageEnvelope<ContributionEarning>
  lifetime_share_quota: number
  contribution_quota: number
}

export interface Contributor {
  user_id: number
  username: string
  display_name: string
  total_count: number
  approved_count: number
  pending_count: number
  contribution_quota: number
  share_percent: number | null
}

// ============================================================================
// Request payloads
// ============================================================================

export interface ContributionModelInput {
  upstream_model: string
  public_model: string
}

export interface SubmitContributionPayload {
  type: number
  name: string
  base_url: string
  key: string
  group: string
  priority: number
  weight: number
  test_model: string
  models: ContributionModelInput[]
}

export interface ReviewDecision {
  model_id: number
  approve: boolean
  public_model: string
  reject_reason: string
}

export interface ReviewContributionPayload {
  id: number
  action: 'approve' | 'reject'
  share_percent?: number | null
  reject_reason?: string
  decisions?: ReviewDecision[]
}

export interface SetSharePercentPayload {
  user_id: number
  share_percent: number | null
}

// ============================================================================
// Wizard: pre-submit upstream fetch + per-model test
// ============================================================================

export interface FetchContributionModelsPayload {
  type: number
  base_url: string
  key: string
}

export interface TestContributionModelPayload {
  type: number
  base_url: string
  key: string
  model: string
}

export interface TestContributionModelResult {
  success: boolean
  message?: string
  time?: number
  error_code?: string
}
