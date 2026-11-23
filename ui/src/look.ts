// The colour mode is the data-theme attribute tokens.css reads, and without it
// a media query follows the system. OLED is the dark ramp in true black.

export type Look = 'system' | 'dark' | 'light' | 'oled'

export const LOOKS: Look[] = ['system', 'dark', 'light', 'oled']
const KEY = 'shiplog-look'

export function storedLook(): Look {
  try {
    const raw = localStorage.getItem(KEY)
    if (LOOKS.includes(raw as Look)) return raw as Look
  } catch {
    // Without storage the page follows the system.
  }
  return 'system'
}

export function applyLook(look: Look): void {
  const root = document.documentElement
  if (look === 'system') root.removeAttribute('data-theme')
  else root.setAttribute('data-theme', look === 'light' ? 'light' : 'dark')
  root.toggleAttribute('data-oled', look === 'oled')
  try {
    localStorage.setItem(KEY, look)
  } catch {
    // The choice then lasts until the page reloads.
  }
}
