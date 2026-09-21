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
/**
 * Utility functions for registration codes
 */

/**
 * Check if a Unix timestamp (in seconds) is expired.
 * @param timestamp - Unix timestamp in seconds (0 means never expires)
 */
export function isTimestampExpired(timestamp: number): boolean {
  if (timestamp === 0) return false
  return timestamp < Date.now() / 1000
}

/**
 * Check if a registration code is expired (enabled + past expiry).
 */
export function isRegistrationCodeExpired(
  expiredTime: number,
  status: number
): boolean {
  return status === 1 && isTimestampExpired(expiredTime)
}

/**
 * Check if a bounded registration code is exhausted.
 * Unlimited codes (max_uses === 0) are never exhausted by count.
 */
export function isCodeExhausted(
  usedCount: number,
  maxUses: number
): boolean {
  return maxUses !== 0 && usedCount >= maxUses
}

/**
 * Build the display/masking value: prefix + code + suffix, all uppercased.
 */
export function formatRegistrationCodeValue(code: RegistrationCodeLike): string {
  return `${code.prefix}${code.code}${code.suffix}`.toUpperCase()
}

/**
 * Mask the middle of a full code value for table display.
 */
export function maskRegistrationCodeValue(fullValue: string): string {
  if (fullValue.length <= 8) return fullValue
  const visible = 4
  return `${fullValue.slice(0, visible)}${'*'.repeat(
    Math.max(fullValue.length - visible * 2, 4)
  )}${fullValue.slice(-visible)}`
}

export type RegistrationCodeLike = {
  prefix: string
  code: string
  suffix: string
}