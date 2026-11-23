// The account page speaks the browser's language where ShipLog has it, as the
// plugin speaks Unraid's, and English otherwise.

export const LOCALES = [
  'ar', 'cs', 'da', 'de', 'el', 'en', 'es', 'fi', 'fr', 'he', 'hu', 'it', 'ja',
  'ko', 'nl', 'no', 'pl', 'pt', 'ro', 'ru', 'sv', 'th', 'tr', 'uk', 'vi', 'zh',
]

const RTL = new Set(['ar', 'he'])

function pick(): string {
  for (const tag of navigator.languages ?? [navigator.language]) {
    const base = tag.toLowerCase().split('-')[0] ?? ''
    const code = base === 'nb' || base === 'nn' ? 'no' : base
    if (LOCALES.includes(code)) return code
  }
  return 'en'
}

const code = pick()
document.documentElement.lang = code
document.documentElement.dir = RTL.has(code) ? 'rtl' : 'ltr'

export const currentLocale = (): string => code
