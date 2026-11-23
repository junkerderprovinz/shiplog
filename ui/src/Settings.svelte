<script lang="ts">
  // The sign-in settings: GlimStone's sign-in tile on the kit's sign-in, under
  // a head with the way back to the status page and the colour mode.
  import { Selector, SignInReauthWindow, SignInTile } from '@glimstone/svelte'
  import { takeSignInCode, type SignInTileCard, type ToastSeverity } from '@glimstone/core'
  import { IDENTITY_PROVIDERS } from '@glimstone/core/identityProviders'
  import { currentLocale } from './i18n'
  import { applyLook, LOOKS, storedLook, type Look } from './look'
  import { signIn } from './session'
  import { reauthText, s, tileText } from './words.svelte'

  const CARDS: SignInTileCard[] = ['password', 'askPassword', 'twoFactor', 'passkeys', 'sessions', 'tokens', 'services']
  const text = $derived(tileText())
  const arrived = takeSignInCode()
  const when = (iso: string) => new Date(iso).toLocaleString(currentLocale())

  let look = $state<Look>(storedLook())
  const looks = $derived(LOOKS.map((id) => ({ id, label: s('look.' + id) })))

  let toast = $state<{ message: string; fail: boolean } | null>(null)
  let toastTimer: ReturnType<typeof setTimeout> | undefined
  function notify(message: string, severity: ToastSeverity): void {
    toast = { message, fail: severity === 'fail' }
    clearTimeout(toastTimer)
    toastTimer = setTimeout(() => (toast = null), 5000)
  }
</script>

<header class="sl-head">
  <a class="sl-brand" href="/"><img src="/logo.svg" alt="" width="26" height="26" /><span>ShipLog</span></a>
  <h1 class="sl-title">{s('page.title')}</h1>
  <a class="sl-back" href="/">{s('page.status')}</a>
</header>

<main class="sl-main">
  <div class="sl-look">
    <Selector
      items={looks}
      value={look}
      label={s('look.theme')}
      inline
      activation="manual"
      onChange={(next: Look) => {
        look = next
        applyLook(next)
      }}
    />
  </div>
  <SignInTile
    api={signIn}
    {text}
    lang={currentLocale()}
    providers={IDENTITY_PROVIDERS}
    cards={CARDS}
    scopes={['read', 'all']}
    {notify}
    {arrived}
    returnTo="/account/"
    time={when}
  />
  <SignInReauthWindow api={signIn} text={reauthText()} />
</main>

<div class="sl-toast" role="status" aria-live="polite">
  {#if toast}<p class:sl-toast-fail={toast.fail}>{toast.message}</p>{/if}
</div>
