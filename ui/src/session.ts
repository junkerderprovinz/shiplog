// The container's sign-in as the account page talks to it.
import { createSignInApi, type SignInOffer } from '@glimstone/core'

export const signIn = createSignInApi({ root: '/' })

export type Offer = SignInOffer

export async function askOffer(): Promise<Offer | null> {
  try {
    const res = await fetch('/api/signin/v1/options', { cache: 'no-store', headers: { Accept: 'application/json' } })
    if (!res.ok || !res.headers.get('content-type')?.includes('application/json')) return null
    return (await res.json()) as Offer
  } catch {
    return null
  }
}

// The line the page shows after a reload, such as the farewell after signing out.
const NOTE = 'shiplog-signin-note'

export function leaveNote(text: string): void {
  try {
    sessionStorage.setItem(NOTE, text)
  } catch {
    // The page then simply says nothing.
  }
}

export function takeNote(): string {
  try {
    const text = sessionStorage.getItem(NOTE) ?? ''
    sessionStorage.removeItem(NOTE)
    return text
  } catch {
    return ''
  }
}
