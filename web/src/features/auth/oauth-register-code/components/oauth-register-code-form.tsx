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
import { zodResolver } from '@hookform/resolvers/zod'
import { useNavigate } from '@tanstack/react-router'
import axios from 'axios'
import { ArrowRight, Loader2 } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { completeOAuthRegistration } from '@/features/auth/api'
import { useAuthRedirect } from '@/features/auth/hooks/use-auth-redirect'
import {
  clearPendingOAuthRegistration,
  readPendingOAuthRegistration,
  type PendingOAuthRegistration,
} from '@/features/auth/lib/oauth-callback-mode'
import { handleServerError } from '@/lib/handle-server-error'
import { AuthOperationError } from '@/lib/secure-verification'
import { createServerError } from '@/lib/server-error-message'
import { cn } from '@/lib/utils'

const registrationCodeSchema = z.object({
  registration_code: z.string().min(1, 'Please enter your registration code'),
})

export function OAuthRegisterCodeForm({
  className,
  ...props
}: React.HTMLAttributes<HTMLFormElement>) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { handleLoginResult } = useAuthRedirect()
  const [isLoading, setIsLoading] = useState(false)
  const pendingRef = useRef<PendingOAuthRegistration | null>(
    readPendingOAuthRegistration()
  )

  const form = useForm<z.infer<typeof registrationCodeSchema>>({
    resolver: zodResolver(registrationCodeSchema),
    defaultValues: { registration_code: '' },
  })

  // No parked sign-up means the user reached this page directly or the token
  // was already spent — send them back to start the flow again.
  useEffect(() => {
    if (!pendingRef.current) {
      toast.error(t('Your sign-up session has expired. Please sign in again.'))
      void navigate({ to: '/sign-in', replace: true })
    }
  }, [navigate, t])

  async function onSubmit(data: z.infer<typeof registrationCodeSchema>) {
    const pending = pendingRef.current
    if (!pending) {
      void navigate({ to: '/sign-in', replace: true })
      return
    }

    setIsLoading(true)
    try {
      const res = await completeOAuthRegistration(
        pending.registerToken,
        data.registration_code.trim()
      )
      if (res?.success) {
        clearPendingOAuthRegistration()
        if (await handleLoginResult(res.data, pending.redirect)) {
          toast.success(t('Signed in successfully!'))
        }
        return
      }
      // A 200 with success:false is a code problem (invalid/used/disabled) —
      // keep the user on this page so they can enter a different code.
      handleServerError(createServerError(res, t('Invalid registration code')))
    } catch (error: unknown) {
      // The parked flow is single-use and short-lived; the server rejects a
      // replayed or expired token with 403. Nothing on this page can recover
      // it, so restart from sign-in.
      if (axios.isAxiosError(error) && error.response?.status === 403) {
        clearPendingOAuthRegistration()
        pendingRef.current = null
        toast.error(t('Your sign-up session has expired. Please sign in again.'))
        void navigate({ to: '/sign-in', replace: true })
        return
      }
      handleServerError(
        AuthOperationError.from(error, t('Invalid registration code'))
      )
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-2', className)}
        {...props}
      >
        <FormField
          control={form.control}
          name='registration_code'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Registration code (required)')}</FormLabel>
              <FormControl>
                <Input
                  placeholder={t('Enter your registration code')}
                  autoComplete='off'
                  autoFocus
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <Button type='submit' className='mt-2' disabled={isLoading}>
          {t('Create account')}
          {isLoading ? <Loader2 className='animate-spin' /> : <ArrowRight />}
        </Button>
      </form>
    </Form>
  )
}
