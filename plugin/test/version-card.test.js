'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const fs = require('node:fs');

const PLUGIN = path.join(__dirname, '..', 'src', 'shiplog', 'usr', 'local', 'emhttp', 'plugins', 'shiplog');
const card = require(path.join(PLUGIN, 'scripts', 'version-card.js'));
const en = JSON.parse(fs.readFileSync(path.join(PLUGIN, 'lang', 'en.json'), 'utf8'));
const ago = () => '3 minutes ago';

test('before the first check the card says it has not looked yet', () => {
  assert.deepEqual(card.updateLine({ kind: 'unchecked' }, en, ago), { line: 'Not checked yet', tone: 'neutral', note: '' });
});

test('up to date needs a check behind it', () => {
  assert.deepEqual(card.updateLine({ kind: 'current' }, en, ago), { line: 'Not checked yet', tone: 'neutral', note: '' });
  assert.deepEqual(card.updateLine({ kind: 'current', checkedAt: 1700000000 }, en, ago),
    { line: 'Up to date', tone: 'ok', note: 'Last checked 3 minutes ago' });
});

test('a found update, a running check and a failed one each say so in their tone', () => {
  assert.deepEqual(card.updateLine({ kind: 'available', version: '3.10.0' }, en, ago),
    { line: 'Update to 3.10.0 available', tone: 'warn', note: '' });
  assert.deepEqual(card.updateLine({ kind: 'checking' }, en, ago), { line: 'Checking …', tone: 'neutral', note: '' });
  assert.deepEqual(card.updateLine({ kind: 'failed', reason: 'offline' }, en, ago), { line: 'Check failed', tone: 'fail', note: 'offline' });
});

test('check for updates waits only while a check runs and goes once an update is found', () => {
  assert.deepEqual(card.checkButton({ kind: 'unchecked' }), { busy: false });
  assert.deepEqual(card.checkButton({ kind: 'current', checkedAt: 1700000000 }), { busy: false });
  assert.deepEqual(card.checkButton({ kind: 'failed' }), { busy: false });
  assert.deepEqual(card.checkButton({ kind: 'checking' }), { busy: true });
  assert.equal(card.checkButton({ kind: 'available', version: '3.10.0' }), null);
});
