/* Where ShipLog's updates stand in the Version card: the words and tone
 * GlimStone's updateLine gives each state, and whether Check for updates
 * shows, as its checkButton decides for a plugin. */
(function (root, factory) {
  if (typeof module === 'object' && module.exports) module.exports = factory();
  else root.ShipLogVersion = factory();
})(this, function () {
  'use strict';

  function updateLine(s, T, ago) {
    switch (s.kind) {
      case 'current':
        // Up to date is a finding, so it needs a check behind it.
        if (!s.checkedAt) break;
        return { line: T.s_vc_current, tone: 'ok', note: T.s_vc_checked.replace('{when}', ago(s.checkedAt)) };
      case 'checking':
        return { line: T.s_vc_checking, tone: 'neutral', note: '' };
      case 'available':
        return { line: T.s_vc_available.replace('{version}', s.version), tone: 'warn', note: '' };
      case 'failed':
        return { line: T.s_vc_failed, tone: 'fail', note: s.reason || '' };
    }
    return { line: T.s_vc_unchecked, tone: 'neutral', note: '' };
  }

  // Once an update is found there is nothing left to look for.
  function checkButton(s) {
    return s.kind === 'available' ? null : { busy: s.kind === 'checking' };
  }

  return { updateLine: updateLine, checkButton: checkButton };
});
