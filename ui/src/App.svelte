<script lang="ts">
  import SignInPage from './SignInPage.svelte'
  import Settings from './Settings.svelte'
  import type { Offer } from './session'

  let { offer }: { offer: Offer } = $props()
</script>

{#if !offer.open && !offer.signedIn}
  <SignInPage {offer} />
{:else}
  <Settings />
{/if}
