// The words GlimStone's sign-in parts take, out of the page's own language
// files, and the codes the kit answers with, each with a sentence of its own.
import {
  componentText,
  listOf,
  SIGN_IN_ERRORS,
  SIGN_IN_REDIRECTS,
  signInTileTextEn,
  type Plural,
  type SignInErrorText,
  type SignInReauthText,
  type SignInRemoved,
  type SignInText,
  type SignInTileText,
} from '@glimstone/core'
import { currentLocale } from './i18n'

type Words = { [key: string]: string | Words }

const FILES = import.meta.glob<{ default: Words }>('./locales/*.json')

let words = $state.raw<Words>({})
let english: Words = {}

async function load(code: string): Promise<Words> {
  const file = (FILES[`./locales/${code}.json`] ?? FILES['./locales/en.json'])!
  return (await file()).default
}

/** Loads the words of the language in use. */
export async function loadWords(): Promise<void> {
  english = await load('en')
  words = await load(currentLocale())
}

function find(from: Words, key: string): string | Words | undefined {
  let at: string | Words | undefined = from
  for (const part of key.split('.')) at = typeof at === 'object' ? at[part] : undefined
  return at
}

/** A sentence of the sign-in, English where the language lacks it, with its {params} filled in. */
export function s(key: string, params?: Record<string, string | number>): string {
  const found = find(words, key) ?? find(english, key)
  const template = typeof found === 'string' ? found : key
  return params ? template.replace(/\{(\w+)\}/g, (m, name: string) => String(params[name] ?? m)) : template
}

// GlimStone fills in {count} where the language files say {n}.
function counted(key: string): Plural {
  const found = find(words, key) ?? find(english, key)
  const forms = typeof found === 'object' ? found : {}
  return Object.fromEntries(Object.entries(forms).map(([k, v]) => [k, String(v).replace('{n}', '{count}')])) as Plural
}

function plural(n: number, key: string): string {
  const forms = counted(key)
  const rule = new Intl.PluralRules(currentLocale()).select(n) as keyof Plural
  return (forms[rule] ?? forms.other ?? '').replace('{count}', String(n))
}

// Codes a browser answers with beside the kit's.
const ERROR_CODES = [...SIGN_IN_ERRORS, 'offline', 'cancelled'] as const

const tile = (key: string) => s('tile.' + key)
const eye = () => {
  const field = componentText(currentLocale()).passwordField
  return { show: field.show, hide: field.hide }
}

export const errorWords = (): SignInErrorText =>
  Object.fromEntries(ERROR_CODES.filter((code) => find(english, 'errors.' + code) !== undefined).map((code) => [code, s('errors.' + code)]))

export const arrivedWords = (): Record<string, string> =>
  Object.fromEntries(SIGN_IN_REDIRECTS.filter((code) => find(english, 'arrived.' + code) !== undefined).map((code) => [code, s('arrived.' + code)]))

export function cardText(): SignInText {
  const field = componentText(currentLocale()).passwordField
  return {
    name: s('name'), password: s('password'), signIn: s('signIn'), signingIn: s('signingIn'), passkey: '', forgot: s('forgot'),
    code: s('codeLabel'), codeHint: s('codeTip'), codeInvalid: s('badCode'), back: s('back'),
    resetCode: s('logCode'), resetCodeTip: s('logCodeTip'), newPassword: s('newPassword'), confirmPassword: field.confirm,
    setPassword: s('setPassword'), saving: s('saving'), mismatch: field.mismatch, tooShort: '', shortHint: s('shortHint'),
    resetRejected: s('resetRejected'), failed: s('failed'), ...eye(),
  }
}

/** What the page after a reset says: the sessions that ended and whatever else the reset took away. */
export function resetNote(removed: SignInRemoved): string {
  const gone = [
    ...(removed.twoFactor ? [tile('password.goes.twoFactor')] : []),
    ...(removed.passkeys ? [plural(removed.passkeys, 'tile.password.goes.passkeys')] : []),
    ...(removed.tokens ? [plural(removed.tokens, 'tile.password.goes.tokens')] : []),
  ]
  return [
    s('resetDone'),
    ...(gone.length ? [s('resetGone', { gone: listOf(currentLocale(), gone) })] : []),
    ...(removed.services ? [s('resetServices')] : []),
  ].join(' ')
}

export function reauthText(): SignInReauthText {
  return {
    title: s('reauth.title'), text: s('reauth.text'), password: s('password'), confirm: s('reauth.confirm'),
    cancel: s('common.cancel'), close: s('common.close'), ...eye(),
    wrong: s('errors.wrongPassword'), off: s('errors.confirmOff'), failed: s('failed'),
  }
}

export function tileText(): SignInTileText {
  const field = componentText(currentLocale()).passwordField
  const days = (n: number) => plural(n, 'tile.tokens.days')
  return {
    cancel: s('common.cancel'), close: s('common.close'), confirm: tile('confirm'), copy: s('common.copy'), copied: s('common.copied'),
    back: s('back'), save: s('common.save'), remove: s('common.remove'), ...eye(), confirmPassword: field.confirm,
    mismatch: field.mismatch, failed: s('failed'),
    needsPassword: { title: tile('needsPassword.title'), reason: tile('needsPassword.reason') },
    notLocal: { title: tile('notLocal.title'), reason: tile('notLocal.reason') },
    password: {
      title: s('password'), hint: tile('password.hint'), openHint: tile('password.openHint'), viaService: tile('password.viaService'), state: s('password'),
      set: tile('password.set'), none: tile('password.none'), change: tile('password.change'), add: tile('password.add'),
      current: tile('password.current'), next: s('newPassword'), save: tile('password.save'), saved: tile('password.saved'),
      first: tile('password.first'), wrongCurrent: tile('password.wrongCurrent'), shortHint: s('shortHint'), shortSaved: s('longer'),
      remove: tile('password.remove'), removeTitle: tile('password.removeTitle'), removeText: tile('password.removeText'),
      goes: {
        twoFactor: tile('password.goes.twoFactor'), passkeys: counted('tile.password.goes.passkeys'),
        services: tile('password.goes.services'), tokens: counted('tile.password.goes.tokens'),
      },
      removed: tile('password.removed'),
    },
    askPassword: {
      title: tile('askPassword.title'), hint: tile('askPassword.hint'), label: tile('askPassword.label'),
      when: tile('askPassword.when'), on: tile('askPassword.on'), off: tile('askPassword.off'),
    },
    twoFactor: {
      title: tile('twoFactor.title'), hint: tile('twoFactor.hint'), state: tile('twoFactor.state'), on: tile('twoFactor.on'),
      off: tile('twoFactor.off'), setUp: tile('twoFactor.setUp'), codesLeft: tile('twoFactor.codes'),
      codesLeftValue: counted('tile.twoFactor.codesLeft'), newCodes: tile('twoFactor.newCodes'), turnOff: tile('twoFactor.turnOff'),
      setupTitle: tile('twoFactor.setupTitle'), scan: tile('twoFactor.scan'), qr: tile('twoFactor.qr'), key: tile('twoFactor.key'),
      code: tile('twoFactor.code'), confirm: tile('twoFactor.turnOn'), sixDigits: tile('twoFactor.sixDigits'),
      codesTitle: tile('twoFactor.codes'), codesText: tile('twoFactor.codesText'), copyCodes: tile('twoFactor.copyCodes'),
      codesSaved: tile('twoFactor.codesSaved'), turnedOn: tile('twoFactor.turnedOn'), newCodesReady: tile('twoFactor.newCodesReady'),
      offTitle: tile('twoFactor.offTitle'), offText: tile('twoFactor.offText'), offConfirm: tile('twoFactor.turnOff'),
      turnedOff: tile('twoFactor.turnedOff'),
    },
    passkeys: {
      title: tile('passkeys.title'), hint: tile('passkeys.hint'), add: tile('passkeys.add'), none: tile('passkeys.none'),
      noneHint: tile('passkeys.noneHint'), created: tile('passkeys.created'), used: tile('passkeys.used'), never: tile('passkeys.never'),
      here: tile('passkeys.here'), other: tile('passkeys.other'), synced: tile('passkeys.synced'), rename: tile('passkeys.rename'),
      renameTitle: tile('passkeys.renameTitle'), removeText: tile('passkeys.removeText'), removed: tile('passkeys.removed'),
      added: tile('passkeys.added'), renamed: tile('passkeys.renamed'), addTitle: tile('passkeys.add'), name: s('name'),
      nameHint: tile('passkeys.nameHint'), fallbackName: tile('passkeys.fallbackName'), continue: tile('passkeys.continue'),
      unavailableTitle: tile('passkeys.unavailableTitle'), unavailable: tile('passkeys.unavailable'),
    },
    sessions: {
      title: tile('sessions.title'), hint: tile('sessions.hint'), current: tile('sessions.current'), end: s('logout'),
      endOthers: tile('sessions.endOthers'), ended: tile('sessions.ended'), othersEnded: tile('sessions.othersEnded'),
      from: tile('sessions.from'), seen: tile('sessions.seen'), on: tile('sessions.on'), browser: tile('sessions.browser'),
    },
    tokens: {
      title: tile('tokens.title'), hint: tile('tokens.hint'), create: tile('tokens.create'), none: tile('tokens.none'),
      noneHint: tile('tokens.noneHint'), scopes: { read: tile('tokens.read'), all: tile('tokens.all') },
      created: tile('tokens.created'), used: tile('tokens.used'), neverUsed: tile('tokens.neverUsed'), expires: tile('tokens.expires'),
      forever: tile('tokens.forever'), endsWhenClosed: tile('tokens.endsWhenClosed'), openHint: tile('tokens.openHint'),
      revoke: tile('tokens.revoke'), revokeText: tile('tokens.revokeText'), revoked: tile('tokens.revoked'),
      newTitle: tile('tokens.create'), name: s('name'), nameHint: tile('tokens.nameHint'), scope: tile('tokens.scope'),
      lifetime: tile('tokens.lifetime'), lifetimes: { '30': days(30), '90': days(90), '365': tile('tokens.year'), never: tile('tokens.never') },
      make: tile('tokens.make'), nameMissing: tile('tokens.nameMissing'), shownTitle: tile('tokens.shownTitle'),
      shownText: tile('tokens.shownText'), shownDone: tile('tokens.shownDone'),
    },
    services: {
      title: tile('services.title'), unnamed: tile('services.unnamed'), hint: tile('services.hint'), add: tile('services.add'),
      on: tile('twoFactor.on'), off: tile('twoFactor.off'), untested: tile('services.untested'), paused: tile('services.paused'),
      show: tile('services.show'), mustTest: tile('services.mustTest'), test: tile('services.test'), testing: tile('services.testing'),
      testOk: tile('services.testOk'), testFailed: tile('services.testFailed'), tested: tile('services.tested'),
      // ShipLog sets up no sign-in service of its own, so this line never shows.
      fromConfig: signInTileTextEn.services.fromConfig,
      confirmAgain: tile('services.confirmAgain'), edit: tile('services.edit'), openBinds: tile('services.openBinds'),
      removeText: tile('services.removeText'), removed: tile('services.removed'), turnedOn: tile('services.turnedOn'),
      turnedOff: tile('services.turnedOff'), updated: tile('services.updated'), pick: tile('services.add'),
      pickHint: tile('services.pickHint'), group: tile('services.group'), own: tile('services.own'), ownSub: 'OpenID Connect',
      ownHint: tile('services.ownHint'), search: tile('services.search'), searchNone: tile('services.searchNone'),
      clear: tile('services.clear'), saved: tile('services.saved'), formHint: tile('services.formHint'),
      whoGetsIn: tile('services.whoGetsIn'), role: tile('services.role'), roleHint: tile('services.roleHint'),
      issuer: tile('services.issuer'), issuerHint: tile('services.issuerHint'), clientId: tile('services.clientId'),
      secret: tile('services.secret'), secretKept: tile('services.secretKept'), redirect: tile('services.redirect'),
      redirectHint: tile('services.redirectHint'), label: tile('services.label'), scopes: tile('services.scopes'),
      nameClaim: tile('services.nameClaim'), groupsClaim: tile('services.groupsClaim'), more: tile('services.more'),
      turnOn: tile('twoFactor.turnOn'), missing: tile('services.missing'), unreachable: tile('services.unreachable'),
    },
    // ShipLog has one account, so the tile never draws the accounts card.
    accounts: signInTileTextEn.accounts,
    roles: { admin: s('role.admin') },
    via: {
      open: tile('via.open'), password: s('password'), passkey: tile('via.passkey'), service: '{name}',
      token: tile('via.token'), door: tile('via.door'),
    },
    errors: errorWords(),
    arrived: arrivedWords(),
  }
}
