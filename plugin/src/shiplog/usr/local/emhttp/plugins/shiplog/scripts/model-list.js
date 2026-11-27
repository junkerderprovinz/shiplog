/* The suggestions of the OpenRouter model field, which takes several IDs
 * separated by commas. A browser offers a datalist entry for the whole field
 * and puts it in place of the whole field, so each entry carries the models
 * listed so far and a pick adds one instead of replacing them. */
(function (root, factory) {
  if (typeof module === 'object' && module.exports) module.exports = factory();
  else root.ShipLogModels = factory();
})(this, function () {
  'use strict';

  // suggestions returns the entries for a field holding value, given the
  // model IDs OpenRouter lists. The part after the last comma is the one being
  // typed, unless it already names a model.
  function suggestions(value, ids) {
    var parts = value.split(',').map(function (p) { return p.trim(); });
    var typed = parts.pop();
    var kept = parts.filter(Boolean);
    if (typed && ids.indexOf(typed) >= 0) kept.push(typed);
    var prefix = kept.length ? kept.join(', ') + ', ' : '';
    return ids.filter(function (id) { return kept.indexOf(id) < 0; })
      .map(function (id) { return prefix + id; });
  }

  return { suggestions: suggestions };
});
