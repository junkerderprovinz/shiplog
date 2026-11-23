import './style.css'
import { createRawSnippet, mount } from 'svelte'
import { setGlyphResolver } from '@glimstone/svelte'
import { wireTooltips } from '@glimstone/core/tooltip'
import { SIGN_IN_GLYPHS } from './glyphs'
import { applyLook, storedLook } from './look'
import { askOffer } from './session'
import { loadWords } from './words.svelte'
import App from './App.svelte'

applyLook(storedLook())
const snippets = Object.fromEntries(
  Object.entries(SIGN_IN_GLYPHS).map(([key, svg]) => [key, createRawSnippet(() => ({ render: () => svg }))]),
)
setGlyphResolver((key) => snippets[key])
wireTooltips()

async function start(root: HTMLElement): Promise<void> {
  const [offer] = await Promise.all([askOffer(), loadWords()])
  if (!offer) {
    location.assign('/')
    return
  }
  mount(App, { target: root, props: { offer } })
}

const root = document.getElementById('app')
if (root) void start(root)
