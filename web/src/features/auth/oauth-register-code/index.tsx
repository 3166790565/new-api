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
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { AuthLayout } from '../auth-layout'
import { OAuthRegisterCodeForm } from './components/oauth-register-code-form'

export function OAuthRegisterCode() {
  const { t } = useTranslation()
  return (
    <AuthLayout>
      <div className='w-full space-y-8'>
        <div className='space-y-3'>
          <h2 className='text-center text-2xl font-semibold tracking-tight sm:text-left'>
            {t('One more step')}
          </h2>
          <p className='text-muted-foreground text-left text-sm sm:text-base'>
            {t(
              'Creating a new account requires a registration code. Enter it below to finish signing up.'
            )}
          </p>
          <p className='text-muted-foreground text-left text-sm sm:text-base'>
            {t('Changed your mind?')}{' '}
            <Link
              to='/sign-in'
              className='hover:text-primary font-medium underline underline-offset-4'
            >
              {t('Back to sign in')}
            </Link>
            .
          </p>
        </div>

        <OAuthRegisterCodeForm className='space-y-0' />
      </div>
    </AuthLayout>
  )
}
