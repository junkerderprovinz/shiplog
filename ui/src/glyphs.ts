// The marks GlimStone's sign-in parts ask for by key, drawn from its glyphs.
import glyphs from '@glimstone/core/glyphs.json'

type Glyph = { name: string; viewBox: string; svg: string }

const byName = new Map((glyphs as Glyph[]).map((g) => [g.name, g]))

export function glyph(name: string, size = 19): string {
  const g = byName.get(name)
  if (!g) throw new Error(`GlimStone has no glyph ${name}`)
  return `<svg width="${size}" height="${size}" viewBox="${g.viewBox}" fill="currentColor" aria-hidden="true">${g.svg}</svg>`
}

export const SIGN_IN_GLYPHS: Record<string, string> = Object.fromEntries(
  Object.entries({
    signIn: 'IconSignIn',
    signInWithPasskey: 'IconKey',
    forgotPassword: 'IconForgotPassword',
    back: 'IconBack',
    setPassword: 'IconKey',
    removePassword: 'IconKeyRevoke',
    edit: 'IconPencil',
    twoFactor: 'IconShieldOn',
    retry: 'IconRetry',
    turnOff: 'IconShieldOff',
    copy: 'IconCopy',
    confirm: 'IconConfirm',
    add: 'IconAdd',
    delete: 'IconTrash',
    signOut: 'IconSignOut',
    revoke: 'IconKeyRevoke',
    test: 'IconSendTest',
    save: 'IconSave',
    block: 'IconShieldOff',
    unblock: 'IconShieldOn',
    passkey: 'IconKey',
    apiToken: 'IconKey',
    account: 'IconAccounts',
    deviceLaptop: 'IconLaptop',
    devicePhone: 'IconPhone',
    forward: 'IconForward',
    'common.close': 'IconClose',
    'common.cancel': 'IconClose',
    'common.copy': 'IconCopy',
  }).map(([key, name]) => [key, glyph(name)]),
)
