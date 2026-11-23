<script lang="ts">
  // The way in: GlimStone's sign-in card on the kit's sign-in. The sign-in
  // service that is on stands under Sign in, and passkeys come through the
  // browser's offer in the password field.
  import { SignInCard, type SignInCardService } from '@glimstone/svelte'
  import { signInCardHandlers, signInServiceLabel, takeSignInCode, type SignInRemoved } from '@glimstone/core'
  import { IDENTITY_PROVIDERS } from '@glimstone/core/identityProviders'
  import { leaveNote, signIn, takeNote, type Offer } from './session'
  import { arrivedWords, cardText, errorWords, resetNote, s } from './words.svelte'

  let { offer }: { offer: Offer } = $props()

  const note = takeNote()
  const code = takeSignInCode()
  const refused = code ? (({ ...errorWords(), ...arrivedWords() }) as Record<string, string | undefined>)[code] ?? s('failed') : undefined

  const text = $derived(cardText())
  const first = $derived(offer.services[0])
  const service = $derived.by<SignInCardService | undefined>(() => {
    if (!first) return undefined
    const { name, mark } = signInServiceLabel(first, IDENTITY_PROVIDERS, { services: { unnamed: s('tile.services.unnamed') } })
    return mark ? { name, mark } : { name }
  })

  // Signed in, people want the status page.
  const handlers = $derived(
    signInCardHandlers(signIn, {
      words: { errors: errorWords(), failed: s('failed') },
      onSignedIn: () => location.assign('/'),
      onReset: (removed: SignInRemoved) => {
        leaveNote(resetNote(removed))
        location.reload()
      },
    }),
  )

  function toService(): void {
    if (first) location.assign(signIn.serviceAddress(first.id, '/'))
  }
</script>

<main class="sl-signin">
  <SignInCard
    logo="/logo.svg"
    appName="ShipLog"
    {text}
    reset
    minPassword={offer.minPassword}
    {note}
    {...refused ? { error: refused } : {}}
    {...service ? { service } : {}}
    onService={toService}
    idPrefix="sl-signin"
    onSignIn={handlers.onSignIn}
    {...offer.passkeys ? { onPasskeyOffer: handlers.onPasskeyOffer } : {}}
    onAskCode={handlers.onAskCode}
    onSetPassword={handlers.onSetPassword}
  />
</main>
