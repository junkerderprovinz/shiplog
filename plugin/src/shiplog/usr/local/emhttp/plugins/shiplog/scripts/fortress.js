/* The Fortress tab: GlimStone's fortress card on ShipLog's engine. Every call
 * goes through status.php, which reaches the engine on its socket and lets a
 * write through only with the page's csrf token. While the engine does not
 * answer, the card stays hidden and the tab says so. */
(function () {
  "use strict";
  var PROXY = "/plugins/shiplog/server/status.php";
  var lang = window.SL_LANG || "en";
  var T = window.SLT;
  var down = document.getElementById("sl-fortress-down");

  function answer(r) {
    return r.json().then(function (d) { return { ok: r.ok, d: d || {} }; }, function () { return { ok: false, d: {} }; });
  }
  function read(query) {
    return fetch(PROXY + "?" + new URLSearchParams(query), { headers: { Accept: "application/json" }, credentials: "same-origin" }).then(answer);
  }
  function send(fields) {
    fields.csrf_token = window.csrf_token || "";
    return fetch(PROXY, { method: "POST", body: new URLSearchParams(fields), credentials: "same-origin" }).then(answer);
  }

  function shape(s) {
    return {
      joined: !!s.joined,
      device: s.device || "",
      removedHere: !!s.removed,
      phraseChanged: !!s.phraseChanged,
      members: s.members || [],
      removed: s.removedMembers || [],
      // Without its place in the fortress ShipLog cannot ask for a backup.
      hints: s.removed || s.phraseChanged ? [{ text: T.s_fortress_no_backup, tone: "warn" }] : []
    };
  }
  // A refusal reaches the card as the code it words, renamed through codes
  // where the card knows it by another name.
  function took(res, codes) {
    if (res.ok) return shape(res.d);
    var code = res.d.error || "";
    throw { code: (codes && codes[code]) || code };
  }

  function when(at) {
    var d = new Date(at);
    if (!at || isNaN(d.getTime())) return "";
    try {
      return d.toLocaleString(lang, { dateStyle: "medium", timeStyle: "short" });
    } catch (e) {
      return d.toLocaleString();
    }
  }

  var adapter = {
    load: function () {
      return read({ path: "fortress" }).then(
        function (res) {
          var up = res.ok && typeof res.d.joined === "boolean";
          down.hidden = up;
          return up ? shape(res.d) : null;
        },
        function () {
          down.hidden = false;
          return null;
        }
      );
    },
    join: function (words) {
      return send({ path: "fortress/join", method: "POST", data: JSON.stringify({ phrase: words.join(" ") }) }).then(function (res) { return took(res); });
    },
    leave: function () {
      return send({ path: "fortress/leave", method: "POST" }).then(function (res) { return took(res); });
    },
    // Out of the fortress, the member is gone from this card all the same.
    remove: function (id) {
      return send({ path: "fortress/member", method: "DELETE", id: id }).then(function (res) { return took(res, { noFortress: "noMember" }); });
    },
    status: function (id) {
      return read({ path: "fortress-member", member: id, lang: lang }).then(function (res) {
        if (!res.ok) throw { code: res.d.error || "" };
        var r = res.d.report || {};
        return {
          report: {
            state: r.state,
            needsYou: r.needsYou || [],
            work: r.work || [],
            next: (r.next || []).map(function (n) { return { title: n.title, at: when(n.at) }; })
          },
          log: (res.d.log || []).map(function (l) { return { at: when(l.at), level: l.level, text: l.text }; })
        };
      });
    }
  };

  window.GlimFortressCard.mount(document.getElementById("sl-fortress"), adapter, { lang: lang, app: { id: "shiplog", name: "ShipLog" }, rows: [] });
})();
