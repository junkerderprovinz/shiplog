'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const fs = require('node:fs');

const PLUGIN = path.join(__dirname, '..', 'src', 'shiplog', 'usr', 'local', 'emhttp', 'plugins', 'shiplog');
const models = require(path.join(PLUGIN, 'scripts', 'model-list.js'));
const ids = ['a/free:free', 'b/one', 'c/two'];

test('an empty field suggests every model as it is', () => {
  assert.deepEqual(models.suggestions('', ids), ids);
});

test('a model being typed is replaced by the pick', () => {
  assert.deepEqual(models.suggestions('b/o', ids), ids);
});

test('a complete model in the field is kept, so a pick adds the next one', () => {
  assert.deepEqual(models.suggestions('b/one', ids), ['b/one, a/free:free', 'b/one, c/two']);
});

test('after a comma the pick joins the models already listed', () => {
  assert.deepEqual(models.suggestions('b/one, c/t', ids), ['b/one, a/free:free', 'b/one, c/two']);
  assert.deepEqual(models.suggestions('b/one,', ids), ['b/one, a/free:free', 'b/one, c/two']);
});

test('a model already listed is not offered twice', () => {
  assert.deepEqual(models.suggestions('b/one, c/two', ids), ['b/one, c/two, a/free:free']);
});

test('the settings page loads the list before its script and fills the field from it', () => {
  const page = fs.readFileSync(path.join(PLUGIN, 'shiplog.page'), 'utf8');
  const tag = (name) => page.indexOf(`<script src="<?autov('/plugins/shiplog/scripts/${name}')?>">`);
  assert.ok(tag('model-list.js') > 0 && tag('model-list.js') < tag('settings.js'));
  const js = fs.readFileSync(path.join(PLUGIN, 'scripts', 'settings.js'), 'utf8');
  assert.match(js, /ShipLogModels\.suggestions\(/);
});
