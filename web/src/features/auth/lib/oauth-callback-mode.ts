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

const OAUTH_POPUP_FLOW_KEY_PREFIX = 'oauth_popup_flow:'

export function rememberOAuthLoginRedirect(
  state: string,
  redirect?: string
): void {
  if (!redirect) return
  try {
    window.sessionStorage.setItem(`oauth_login_redirect:${state}`, redirect)
  } catch {
    // Login can still complete using the default destination.
  }
}

export function consumeOAuthLoginRedirect(state: string): string | null {
  try {
    const key = `oauth_login_redirect:${state}`
    const redirect = window.sessionStorage.getItem(key)
    window.sessionStorage.removeItem(key)
    return redirect
  } catch {
    return null
  }
}

const OAUTH_REGISTER_PENDING_KEY = 'oauth_register_pending'

/**
 * A deferred OAuth sign-up parked by the callback: the provider identity was
 * already verified server-side under `registerToken`, and the standalone
 * registration-code page reads this to finish creating the account.
 */
export interface PendingOAuthRegistration {
  registerToken: string
  provider: string
  redirect?: string
}

/**
 * Persist a deferred OAuth sign-up so the registration-code page can complete
 * it. The register token is short-lived and single-use server-side, so storing
 * it in `sessionStorage` keeps it scoped to this tab and cleared on close.
 */
export function rememberPendingOAuthRegistration(
  pending: PendingOAuthRegistration
): boolean {
  if (!pending.registerToken) return false
  try {
    window.sessionStorage.setItem(
      OAUTH_REGISTER_PENDING_KEY,
      JSON.stringify(pending)
    )
    return true
  } catch {
    return false
  }
}

/** Read the parked OAuth sign-up without clearing it (survives page reloads). */
export function readPendingOAuthRegistration(): PendingOAuthRegistration | null {
  try {
    const raw = window.sessionStorage.getItem(OAUTH_REGISTER_PENDING_KEY)
    if (!raw) return null
    const parsed: unknown = JSON.parse(raw)
    if (
      !parsed ||
      typeof parsed !== 'object' ||
      typeof (parsed as PendingOAuthRegistration).registerToken !== 'string' ||
      typeof (parsed as PendingOAuthRegistration).provider !== 'string'
    ) {
      return null
    }
    return parsed as PendingOAuthRegistration
  } catch {
    return null
  }
}

/** Drop the parked OAuth sign-up once it is completed or abandoned. */
export function clearPendingOAuthRegistration(): void {
  try {
    window.sessionStorage.removeItem(OAUTH_REGISTER_PENDING_KEY)
  } catch {
    // Nothing to clean up if storage is unavailable.
  }
}

/** Minimal shape of `sessionStorage`, kept structural so tests can fake it. */
export interface OAuthModeStorage {
  getItem: (key: string) => string | null
  setItem: (key: string, value: string) => void
}

/** Minimal owner shape for safely accessing `sessionStorage`. */
export interface OAuthSessionStorageOwner {
  readonly sessionStorage: OAuthModeStorage
}

/** Minimal shape of `window.opener`. */
export interface OAuthModeOpener {
  closed: boolean
}

export interface OAuthCallbackModeContext {
  opener: OAuthModeOpener | null | undefined
  storage: OAuthModeStorage | null | undefined
}

export type OAuthCallbackMode = 'login' | 'bind' | 'verify'

/**
 * Access `sessionStorage` without letting browser privacy settings crash the
 * OAuth page or binding action.
 */
export function getOAuthSessionStorage(
  owner: OAuthSessionStorageOwner | null | undefined
): OAuthModeStorage | null {
  try {
    return owner?.sessionStorage ?? null
  } catch {
    return null
  }
}

/**
 * Stamp a freshly opened, still same-origin popup as an OAuth popup flow.
 * Call this before navigating the popup to the provider.
 */
export function markOAuthPopup(
  storage: OAuthModeStorage | null | undefined,
  provider: string,
  state: string,
  intent: 'bind' | 'verify'
): boolean {
  if (!storage || !provider || !state) return false

  try {
    const key = `${OAUTH_POPUP_FLOW_KEY_PREFIX}${provider}`
    const marker = JSON.stringify({ state, intent })
    storage.setItem(key, marker)
    return storage.getItem(key) === marker
  } catch {
    return false
  }
}

/**
 * Resolve how a callback on `/oauth/:provider` should be handled.
 *
 * A bind requires all three pieces of evidence: our own stamp for this exact
 * provider and state, plus a live opener to hand the result back to. Anything
 * else is a login, which is also the safe default — a login callback recovers
 * on its own, while a wrongly assumed bind can only time out.
 */
export function resolveOAuthCallbackMode(
  provider: string,
  state: string,
  { opener, storage }: OAuthCallbackModeContext
): OAuthCallbackMode {
  if (!opener || opener.closed || !storage || !state) return 'login'

  try {
    const value = storage.getItem(`${OAUTH_POPUP_FLOW_KEY_PREFIX}${provider}`)
    if (!value) return 'login'
    const marker: unknown = JSON.parse(value)
    if (
      !marker ||
      typeof marker !== 'object' ||
      !('state' in marker) ||
      !('intent' in marker)
    ) {
      return 'login'
    }
    if (marker.state !== state) return 'login'
    if (marker.intent === 'bind' || marker.intent === 'verify') {
      return marker.intent
    }
  } catch {
    return 'login'
  }
  return 'login'
}
