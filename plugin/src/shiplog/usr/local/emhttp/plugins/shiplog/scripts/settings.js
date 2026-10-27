/* ShipLog's settings page: the tabs, the switches, the checks, the auto-update
 * container list, the info bubbles and the label engine for its buttons.
 *
 * The page reads as Unraid's own without TowerTorch. With TowerTorch, which
 * stamps cc-tools-on on <html> for every settings page, its sheets draw the
 * fields, buttons and switches, and this script hands them the colour mode and
 * the label mode TowerTorch keeps in localStorage. */
(function () {
  "use strict";
  var root = document.getElementById("sl-cfg");
  var form = root.querySelector("form");
  // The give windows sit in <body>, clear of any stacking context on the page.
  var wins = document.getElementById("sl-wins");
  document.body.appendChild(wins);
  var html = document.documentElement;
  var T = window.SLT;
  var GLYPH = window.SL_GLYPHS;

  function get(k, d) { try { var v = localStorage.getItem(k); return v == null ? d : v; } catch (e) { return d; } }
  function put(k, v) { try { localStorage.setItem(k, v); } catch (e) {} }
  function byId(id) { return document.getElementById(id); }
  function themed() { return html.classList.contains("cc-tools-on"); }

  /* The bubble: one element in <body>, shared by every (i), by a button that
     shows only its glyph and by a tab that does. It opens below its trigger,
     flips above when below would clip, stays inside the window and points at
     the trigger with its arrow. */
  var bubble = null, tipFor = null, pointerLast = false;
  function bubbleEl() {
    if (!bubble) {
      bubble = document.createElement("div");
      bubble.id = "sl-bubble";
      bubble.className = "sl-bubble";
      bubble.setAttribute("role", "tooltip");
      bubble.style.display = "none";
      document.body.appendChild(bubble);
    }
    return bubble;
  }
  function hideTip() { if (bubble) bubble.style.display = "none"; tipFor = null; }
  function showTip(el) {
    var text = el.getAttribute("data-sl-tip");
    if (!text) return;
    var b = bubbleEl(), r = el.getBoundingClientRect();
    b.textContent = text;
    b.style.display = "block";
    var vw = html.clientWidth || window.innerWidth, vh = html.clientHeight || window.innerHeight;
    var w = b.offsetWidth, h = b.offsetHeight, cx = r.left + r.width / 2;
    var x = Math.max(8 + w / 2, Math.min(vw - 8 - w / 2, cx));
    var above = r.bottom + 8 + h > vh && r.top - 8 - h >= 0;
    b.classList.toggle("sl-bubble-above", above);
    b.style.left = x + "px";
    b.style.top = (above ? r.top - 8 - h : r.bottom + 8) + "px";
    b.style.setProperty("--sl-tip-ax", Math.max(10, Math.min(w - 10, cx - (x - w / 2))) + "px");
    tipFor = el;
  }
  function tipTarget(e) { return e.target && e.target.closest ? e.target.closest("[data-sl-tip]") : null; }
  document.addEventListener("mouseover", function (e) { var t = tipTarget(e); if (t && t !== tipFor) showTip(t); });
  document.addEventListener("mouseout", function (e) {
    if (!tipFor) return;
    if (e.relatedTarget instanceof Node && tipFor.contains(e.relatedTarget)) return;
    hideTip();
  });
  // Focus that follows a click is a side effect of the click, so only the
  // keyboard opens a bubble on focus.
  document.addEventListener("focusin", function (e) { if (pointerLast) return; var t = tipTarget(e); if (t) showTip(t); });
  document.addEventListener("focusout", function () { if (tipFor) hideTip(); });
  document.addEventListener("pointerdown", function () { pointerLast = true; hideTip(); }, true);
  document.addEventListener("keydown", function (e) {
    pointerLast = false;
    if (e.key === "Escape" && tipFor) hideTip();
  }, true);
  window.addEventListener("scroll", hideTip, true);

  /* Every setting saves itself (GlimStone, "Saving"): a switch or a pick from
     a list at once, a typed value once its field has been quiet for 800ms. A
     save posts the whole form to Unraid's update.php, which merges it into
     shiplog.cfg, and restarts the engine only when a key it reads at start
     changed. Changes made while a save runs go out together in the next one,
     so a burst of edits restarts the engine once. A switch or a pick that could
     not be saved goes back and shakes; a typed value stays as it is. */
  var UI_ONLY = { CONFIRM_UPDATE: true, SILENT_UPDATE: true };
  var savedEl = byId("sl-saved");
  var saving = false, queued = null, typed = null, typeTimer = 0;
  function note(state, text) { savedEl.setAttribute("data-state", state); savedEl.textContent = text; }
  function job(names, undo, ctl) {
    return { restart: names.some(function (n) { return !UI_ONLY[n]; }), undo: undo ? [undo] : [], ctl: ctl ? [ctl] : [] };
  }
  function merged(a, b) {
    if (!a) return b;
    return { restart: a.restart || b.restart, undo: a.undo.concat(b.undo), ctl: a.ctl.concat(b.ctl) };
  }
  function save(j) {
    if (saving) { queued = merged(queued, j); return; }
    saving = true;
    note("run", j.restart ? T.s_saving_restart : T.s_saving);
    // update.php never answers a multipart body, so the form goes URL-encoded.
    var body = new URLSearchParams(new FormData(form));
    if (!j.restart) { body.delete("#command"); body.delete("#arg[1]"); }
    fetch("/update.php", { method: "POST", body: body, credentials: "same-origin" })
      .then(function (r) {
        // An expired session answers with the login page.
        if (!r.ok || r.redirected) throw new Error(r.ok ? T.s_session : "HTTP " + r.status);
        var t = new Date().toLocaleTimeString(window.SL_LANG, { hour: "2-digit", minute: "2-digit" });
        note("ok", T.s_saved + " · " + t);
        if (j.restart) loadStatus();
      })
      .catch(function (e) {
        j.undo.forEach(function (fn) { fn(); });
        j.ctl.forEach(fail);
        note("err", T.s_save_failed.replace("%s", function () { return e.message; }));
      })
      .then(function () {
        saving = false;
        if (queued) { var q = queued; queued = null; save(q); }
      });
  }
  function isText(el) { return el.tagName === "INPUT" && (el.type === "text" || el.type === "password"); }
  // A typed value goes out once per change: after the pause, or when the field
  // is left or Enter is pressed before the pause ends.
  function flushTyped() {
    clearTimeout(typeTimer);
    if (!typed) return;
    var changed = typed.filter(function (f) { return f.value !== f.getAttribute("data-sl-saved"); });
    typed = null;
    if (!changed.length) return;
    changed.forEach(function (f) { f.setAttribute("data-sl-saved", f.value); });
    var j = job(changed.map(function (f) { return f.name; }));
    j.ctl = changed;
    save(j);
  }
  form.querySelectorAll("input.sl-f").forEach(function (f) { f.setAttribute("data-sl-saved", f.value); });
  form.querySelectorAll("select.sl-sel").forEach(function (s) { s.setAttribute("data-sl-saved", s.value); });
  form.addEventListener("input", function (e) {
    var f = e.target;
    if (!isText(f) || !f.name) return;
    typed = typed || [];
    if (typed.indexOf(f) < 0) typed.push(f);
    clearTimeout(typeTimer);
    typeTimer = setTimeout(flushTyped, 800);
  });
  form.addEventListener("submit", function (e) { e.preventDefault(); flushTyped(); });
  form.addEventListener("change", function (e) {
    var el = e.target;
    if (isText(el)) { flushTyped(); return; }
    if (el.tagName === "SELECT" && el.name) {
      var was = el.getAttribute("data-sl-saved");
      el.setAttribute("data-sl-saved", el.value);
      save(job([el.name], function () {
        el.value = was;
        el.setAttribute("data-sl-saved", was);
        if (el.id === "sl-au-mode") auMode();
      }, el));
    } else if (el.hasAttribute("data-sl-mirror")) {
      save(job([byId(el.getAttribute("data-sl-mirror")).name], function () {
        el.checked = !el.checked;
        mirror(el);
        if (el.id === "sl-au-en-cb") auShown();
      }, el));
    } else if (el.closest("#sl-au-ct")) {
      save(job(["AUTOUPDATE_EXCLUDE_CONTAINERS"], function () { el.checked = !el.checked; auSync(el); }, el));
    }
  });

  /* A switch is a checkbox without a name; the hidden field beside it posts
     true or false. */
  function mirror(cb) {
    var h = byId(cb.getAttribute("data-sl-mirror"));
    if (h) h.value = cb.checked ? "true" : "false";
  }
  root.querySelectorAll("input[data-sl-mirror]").forEach(function (cb) {
    cb.addEventListener("change", function () {
      mirror(cb);
      if (cb.id === "sl-au-en-cb") auShown();
    });
  });

  /* Tabs: a vertical tab list, arrows move between the tabs, the choice is
     kept for the next visit. */
  var tabs = Array.prototype.slice.call(root.querySelectorAll(".sl-tab"));
  function openTab(tab, focus) {
    var name = tab.getAttribute("data-sl-tab");
    tabs.forEach(function (t) {
      var on = t === tab;
      t.setAttribute("aria-selected", on ? "true" : "false");
      t.tabIndex = on ? 0 : -1;
      var p = byId(t.getAttribute("aria-controls"));
      if (p) p.hidden = !on;
    });
    put("shiplog.settings.tab", name);
    hideTip();
    if (focus) tab.focus();
    fit();
    paint();
  }
  tabs.forEach(function (t, i) {
    t.addEventListener("click", function () { openTab(t, false); });
    t.addEventListener("keydown", function (e) {
      var n = { ArrowDown: i + 1, ArrowRight: i + 1, ArrowUp: i - 1, ArrowLeft: i - 1, Home: 0, End: tabs.length - 1 }[e.key];
      if (n === undefined) return;
      e.preventDefault();
      openTab(tabs[(n + tabs.length) % tabs.length], true);
    });
  });
  // Below 1024px the tabs show their glyphs only, and the name moves into the bubble.
  var narrow = window.matchMedia("(max-width: 1023px)");
  function tabTips() {
    tabs.forEach(function (t) {
      var name = t.querySelector(".sl-tablbl").textContent;
      if (narrow.matches) t.setAttribute("data-sl-tip", name); else t.removeAttribute("data-sl-tip");
    });
  }
  if (narrow.addEventListener) narrow.addEventListener("change", tabTips); else if (narrow.addListener) narrow.addListener(tabTips);
  tabTips();

  /* The eye inside a secret field shows and hides the value. */
  root.querySelectorAll("[data-sl-eye]").forEach(function (eye) {
    function flip() {
      var f = byId(eye.getAttribute("data-sl-eye"));
      var show = f.type === "password";
      f.type = show ? "text" : "password";
      eye.innerHTML = show ? GLYPH.EyeOff : GLYPH.Eye;
      eye.setAttribute("aria-label", show ? T.s_hide : T.s_show);
      eye.setAttribute("aria-pressed", show ? "true" : "false");
    }
    eye.addEventListener("click", flip);
    eye.addEventListener("keydown", function (e) { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); flip(); } });
  });

  /* The label engine. TowerTorch keeps one mode for every button it draws as
     a badge (text, textGlyph, glyph or reactive) and puts it on <html>; this
     page follows it, and without TowerTorch shows glyph and text. A button
     keeps one width in all four modes, so a mode change or a reactive reveal
     never moves the row: at least the stage its label belongs to, and at least
     what it measures with glyph and words. */
  var MODES = ["text", "textGlyph", "glyph", "reactive"];
  function labelWidth(s) {
    var n = 0;
    for (var i = 0; i < s.length; i++) {
      var c = s.charCodeAt(i);
      var wide = (c >= 0x1100 && c <= 0x115f) || (c >= 0x2e80 && c <= 0xa4cf) || (c >= 0xac00 && c <= 0xd7a3) ||
        (c >= 0xf900 && c <= 0xfaff) || (c >= 0xfe30 && c <= 0xfe6f) || (c >= 0xff00 && c <= 0xff60) || (c >= 0xffe0 && c <= 0xffe6);
      n += wide ? 2 : 1;
    }
    return n;
  }
  function stage(w) { return w <= 10 ? "xs" : w <= 16 ? "sm" : w <= 26 ? "md" : "lg"; }
  function mode() {
    var m = themed() ? html.getAttribute("data-cc-labels-buttons") : "";
    return MODES.indexOf(m) < 0 ? "textGlyph" : m;
  }
  function labels() {
    var m = mode();
    if (root.getAttribute("data-sl-labels") !== m) root.setAttribute("data-sl-labels", m);
    // The coin tiles in the crypto window follow the same mode.
    wins.setAttribute("data-sl-labels", m);
    root.querySelectorAll(".sl-btn").forEach(function (b) {
      var text = b.querySelector(".sl-l").textContent.trim();
      var st = "sl-w-" + stage(labelWidth(text));
      ["sl-w-xs", "sl-w-sm", "sl-w-md", "sl-w-lg"].forEach(function (c) { b.classList.toggle(c, c === st); });
      b.style.setProperty("--sl-rchars", String(labelWidth(text)));
      if (m === "glyph" || (m === "reactive" && b.disabled)) b.setAttribute("data-sl-tip", text);
      else b.removeAttribute("data-sl-tip");
    });
    fit();
  }
  // Measured in textGlyph. A button in a closed tab has no box yet and is
  // measured when its tab opens.
  function fit() {
    var m = root.getAttribute("data-sl-labels");
    root.classList.add("sl-still");
    root.setAttribute("data-sl-labels", "textGlyph");
    var btns = Array.prototype.filter.call(root.querySelectorAll(".sl-btn"), function (b) { return b.getClientRects().length > 0; });
    btns.forEach(function (b) { b.style.removeProperty("--sl-w"); });
    var ws = btns.map(function (b) { return Math.ceil(b.getBoundingClientRect().width); });
    root.setAttribute("data-sl-labels", m);
    btns.forEach(function (b, i) { b.style.setProperty("--sl-w", ws[i] + "px"); });
    // Settled before the transitions come back, or a reactive label would replay its opening.
    void root.offsetWidth;
    root.classList.remove("sl-still");
  }

  /* The colour engine, read from TowerTorch: the accent, the rainbow with one
     position per tab, per card heading and per switch and the first for the
     banner, and the reactive mode in which an idle tab glyph rests grey.
     Without TowerTorch nothing is set and the page keeps Unraid's colours. */
  function ink(hex) {
    if (window.CCTheme && window.CCTheme.idealText) return window.CCTheme.idealText(hex);
    return "#161616";
  }
  function hue(el, c, a, b) {
    if (c) { el.style.setProperty(a, c); el.style.setProperty(b, ink(c)); }
    else { el.style.removeProperty(a); el.style.removeProperty(b); }
  }
  function paint() {
    var on = themed() && get("cc.theming", "1") !== "0";
    var rb = on && get("cc.rainbow", "0") === "1" && window.CCTheme && window.CCTheme.rbColor;
    var accent = (getComputedStyle(html).getPropertyValue("--cc-accent") || "").trim() || "#FCC419";
    root.classList.toggle("sl-reactive", on && get("cc.rbmode", "all") === "active");
    function at(i) { return rb ? window.CCTheme.rbColor(i, accent) : ""; }
    // In the give windows the title, each option and each chain take their own
    // position, and a coin tile its own once it is picked.
    wins.querySelectorAll(".sl-win-title").forEach(function (t) { hue(t, at(0), "--sl-hue", "--sl-hue-ink"); });
    wins.querySelectorAll(".sl-seg, .sl-nets").forEach(function (g) {
      Array.prototype.forEach.call(g.children, function (b, i) { hue(b, at(i), "--sl-hue", "--sl-hue-ink"); });
    });
    wins.querySelectorAll(".sl-coin").forEach(function (b, i) { hue(b, b.getAttribute("aria-selected") === "true" ? at(i) : "", "--sl-hue", "--sl-hue-ink"); });
    var banner = root.querySelector(".sl-hero");
    if (rb) banner.style.setProperty("--sl-hero-rb", at(0)); else banner.style.removeProperty("--sl-hero-rb");
    tabs.forEach(function (t, i) { hue(t, at(i), "--sl-hue", "--sl-hue-ink"); });
    root.querySelectorAll(".sl-pane").forEach(function (p) {
      p.querySelectorAll(".sl-card").forEach(function (c, i) { hue(c, at(i), "--sl-hue", "--sl-hue-ink"); });
      // TowerTorch draws these checkboxes as its switches and fills a checked
      // one from --cc-rb-c.
      p.querySelectorAll("input.sl-sw").forEach(function (s, i) { hue(s, at(i), "--cc-rb-c", "--cc-rb-ct"); });
    });
  }
  function engines() { labels(); paint(); }
  new MutationObserver(engines).observe(html, { attributes: true, attributeFilter: ["class", "data-cc-labels-buttons"] });
  window.addEventListener("storage", function (e) { if (!e.key || /^cc\./.test(e.key)) engines(); });

  /* The checks. A button that runs a test shows the verdict on itself, a tick
     or a cross in the state colour, and the reason stands in one line under
     its row until the next attempt. */
  function fail(btn) {
    btn.classList.remove("sl-shake");
    void btn.offsetWidth; // restarts the animation on a second failure
    btn.classList.add("sl-shake");
  }
  function verdict(btn, out, ok, msg) {
    btn.classList.toggle("sl-ok", ok);
    btn.classList.toggle("sl-err", !ok);
    var g = btn.querySelector(".sl-g"); if (g) g.innerHTML = ok ? GLYPH.Check : GLYPH.Close;
    out.className = "sl-res " + (ok ? "ok" : "err");
    out.textContent = msg || "";
    btn.removeAttribute("aria-busy");
    if (!ok) fail(btn);
  }
  function val(n) { var e = form.querySelector('[name="' + n + '"]'); return e ? e.value : ""; }
  function setVal(n, v) {
    var e = form.querySelector('[name="' + n + '"]');
    e.value = v;
    e.dispatchEvent(new Event("input", { bubbles: true }));
    e.dispatchEvent(new Event("change", { bubbles: true }));
  }
  var CHECKS = {
    github: function () { return "github-check.php?token=" + encodeURIComponent(val("GITHUB_TOKEN")); },
    dockerhub: function () { return "dockerhub-check.php?user=" + encodeURIComponent(val("DOCKERHUB_USERNAME")) + "&token=" + encodeURIComponent(val("DOCKERHUB_TOKEN")); },
    ollama: function () { return "ollama-check.php?url=" + encodeURIComponent(val("OLLAMA_URL")) + "&model=" + encodeURIComponent(val("OLLAMA_MODEL")); },
    "ollama-detect": function () { return "ollama-detect.php?url=" + encodeURIComponent(val("OLLAMA_URL")); },
    llamaswap: function () { return "llamaswap-check.php?url=" + encodeURIComponent(val("LLAMASWAP_URL")) + "&model=" + encodeURIComponent(val("LLAMASWAP_MODEL")); },
    matrix: function () {
      return "matrix-check.php?hs=" + encodeURIComponent(val("MATRIX_HOMESERVER")) + "&token=" + encodeURIComponent(val("MATRIX_TOKEN")) +
        "&room=" + encodeURIComponent(val("MATRIX_ROOM"));
    },
    apprise: function () { return "apprise-test.php"; }
  };
  // The Apprise URL carries its key, so it goes in a POST body rather than a query string.
  function post(kind) {
    if (kind !== "apprise") return undefined;
    return { method: "POST", body: new URLSearchParams({ url: val("APPRISE_URL"), csrf_token: window.csrf_token || "" }) };
  }
  root.querySelectorAll("[data-sl-check]").forEach(function (btn) {
    var kind = btn.getAttribute("data-sl-check"), out = byId(btn.getAttribute("data-sl-out"));
    var glyph = btn.querySelector(".sl-g") ? btn.querySelector(".sl-g").innerHTML : "";
    btn.addEventListener("click", function () {
      btn.classList.remove("sl-ok", "sl-err");
      if (btn.querySelector(".sl-g")) btn.querySelector(".sl-g").innerHTML = glyph;
      btn.setAttribute("aria-busy", "true");
      out.className = "sl-res run";
      out.textContent = (kind === "ollama-detect" ? T.s_autodetect : kind === "apprise" ? T.s_apprise_test : T.s_check) + " …";
      fetch("/plugins/shiplog/server/" + CHECKS[kind](), post(kind))
        .then(function (r) { return r.json(); })
        .then(function (d) {
          verdict(btn, out, !!d.ok, d.message);
          if (kind === "ollama-detect" && d.ok && d.url) {
            setVal("OLLAMA_URL", d.url);
            if (!val("OLLAMA_MODEL") && d.models && d.models.length) setVal("OLLAMA_MODEL", d.models[0]);
          }
        })
        .catch(function (e) { verdict(btn, out, false, String(e)); });
    });
  });

  function infoIcon(text) {
    var i = document.createElement("span");
    i.className = "sl-info"; i.tabIndex = 0; i.setAttribute("role", "img");
    i.setAttribute("aria-label", text); i.setAttribute("data-sl-tip", text);
    i.innerHTML = root.querySelector(".sl-info").innerHTML;
    return i;
  }

  /* The engine's state and the address of its status page. The state is read
     again after a save that restarted the engine. */
  var statusLink = byId("sl-statuslink");
  if (!statusLink.getAttribute("href")) statusLink.href = "http://" + (window.SL_STATUS_HOST || location.hostname) + ":" + statusLink.getAttribute("data-sl-port") + "/";
  function loadStatus() {
    var stat = byId("sl-stat"), txt = byId("sl-stat-txt");
    function show(state, text, hint) {
      stat.setAttribute("data-state", state);
      txt.textContent = String(text || "").replace(/^[✓✗]\s*/, "");
      var old = stat.querySelector(".sl-info"); if (old) old.remove();
      if (hint) stat.appendChild(infoIcon(String(hint).replace(/^\(|\)$/g, "")));
    }
    fetch("/plugins/shiplog/server/status.php", { headers: { Accept: "application/json" } })
      .then(function (r) { return r.json().then(function (d) { return { ok: r.ok, d: d }; }, function () { return { ok: false, d: null }; }); })
      .then(function (res) {
        if (res.ok && Array.isArray(res.d)) {
          show("ok", T.s_running.replace("%n", res.d.length));
          var nv = Object.create(null);
          res.d.forEach(function (s) { var n = s && s.container && s.container.name; if (n && s.newer_version) nv[n] = String(s.newer_version); });
          auLoaded(res.d.map(function (s) { return s && s.container && s.container.name; }), nv);
        } else {
          show("err", T.s_unreachable, T.s_unreachable_hint);
          auLoaded(null);
        }
      })
      .catch(function () { show("err", T.s_unreachable, T.s_unreachable_hint); auLoaded(null); });
  }
  loadStatus();

  /* Auto-update. Everything below the main switch only matters while it is on,
     so those rows are left out while it is off. */
  function auShown() {
    var on = byId("sl-au-en").value === "true";
    root.querySelectorAll("[data-sl-au]").forEach(function (r) { r.hidden = !on; });
    auState();
  }
  function auMode() {
    var m = byId("sl-au-mode").value;
    var t = byId("sl-au-time"), e = byId("sl-au-every"), u = byId("sl-au-every-unit");
    var unit = m === "hours" ? T.s_au_unit_hours : (m === "days" ? T.s_au_unit_days : "");
    t.hidden = m !== "daily";
    e.hidden = u.hidden = !(m === "hours" || m === "days");
    // An empty slot would keep its gap and hold the dropdown off the edge.
    t.parentNode.hidden = t.hidden && e.hidden;
    u.textContent = unit;
    e.setAttribute("aria-label", unit);
  }
  byId("sl-au-mode").addEventListener("change", auMode);
  auMode();

  // One switch per container the engine tracks; on means auto-update may
  // update it. The hidden field holds the names switched off, including one
  // with no container behind it right now, so Apply never drops an exclusion.
  var skip = [];
  var names = null; // null until the engine answered, or when it is down
  var newer = Object.create(null); // name -> newer version of a pinned container
  var loadState = "loading";
  var NAME_OK = /^[A-Za-z0-9][A-Za-z0-9_.-]*$/; // Docker's name rule keeps the cfg value safe
  function key(s) { return String(s).toLowerCase(); }
  function auLoaded(list, nv) {
    var seen = {};
    names = Array.isArray(list) ? list.filter(function (n) {
      if (!n || !NAME_OK.test(n) || seen[n]) return false;
      seen[n] = true; return true;
    }) : null;
    loadState = names ? "ok" : "down";
    newer = (names && nv) ? nv : Object.create(null);
    auRender();
  }
  function auItem(name, on, gone, nv) {
    if (nv) return auPinned(name, on, nv);
    var l = document.createElement("label"); l.className = "sl-ct-item";
    var cb = document.createElement("input"); cb.type = "checkbox"; cb.className = "sl-sw"; cb.checked = on;
    cb.setAttribute("role", "switch");
    cb.setAttribute("data-name", name); cb.setAttribute("data-gone", gone ? "1" : "0");
    cb.addEventListener("change", function () { auSync(cb); });
    var t = document.createElement("span"); t.className = "sl-ct-name"; t.textContent = name;
    l.appendChild(cb); l.appendChild(t);
    if (gone) { var g = document.createElement("small"); g.textContent = T.s_au_ct_gone; l.appendChild(g); }
    return l;
  }
  // A pinned container has nothing for auto-update to apply, so it gets the
  // newer-version hint instead of a switch. An exclusion it already has stays
  // in data-off.
  function auPinned(name, on, nv) {
    var d = document.createElement("div"); d.className = "sl-ct-item sl-ct-pin";
    d.setAttribute("data-name", name); d.setAttribute("data-off", on ? "0" : "1");
    var ring = document.createElement("span"); ring.className = "sl-ct-ring"; ring.setAttribute("aria-hidden", "true");
    var t = document.createElement("span"); t.className = "sl-ct-name"; t.textContent = name;
    var hint = T.s_au_ct_newer.replace("%v", function () { return nv; });
    d.appendChild(ring); d.appendChild(t); d.appendChild(infoIcon(hint));
    return d;
  }
  // Excluded containers come first, so none hides below the fold.
  function auRender() {
    var box = byId("sl-au-ct");
    var off = {}; skip.forEach(function (n) { off[key(n)] = true; });
    var have = {}, rows = [];
    (names || []).forEach(function (n) { have[key(n)] = true; rows.push({ name: n, off: !!off[key(n)], gone: false }); });
    skip.forEach(function (n) { if (!have[key(n)]) rows.push({ name: n, off: true, gone: names !== null }); });
    rows.sort(function (a, b) { return (b.off - a.off) || key(a.name).localeCompare(key(b.name)); });
    box.textContent = "";
    rows.forEach(function (r) { box.appendChild(auItem(r.name, !r.off, r.gone, r.gone ? "" : newer[r.name])); });
    auState();
    paint();
  }
  function auSync(changed) {
    var box = byId("sl-au-ct"), out = [];
    // Switching a stale exclusion back on simply drops it.
    if (changed.checked && changed.getAttribute("data-gone") === "1") changed.closest(".sl-ct-item").remove();
    box.querySelectorAll(".sl-ct-item").forEach(function (it) {
      var cb = it.querySelector("input[type=checkbox]");
      if (cb ? !cb.checked : it.getAttribute("data-off") === "1") out.push((cb || it).getAttribute("data-name"));
    });
    skip = out;
    byId("sl-au-skip").value = out.join(",");
    auState();
  }
  function auState() {
    var box = byId("sl-au-ct"), hint = byId("sl-au-ct-hint");
    var msg = "";
    if (loadState === "loading") msg = T.s_au_ct_loading;
    else if (loadState === "down") msg = T.s_unreachable + " " + T.s_unreachable_hint;
    else if (!box.children.length) msg = T.s_au_ct_none;
    hint.textContent = String(msg || "").replace(/^[✓✗]\s*/, "");
    hint.hidden = !msg;
  }
  (function () {
    var seen = {};
    skip = byId("sl-au-skip").value.split(",").map(function (s) { return s.trim(); }).filter(function (n) {
      if (!n || seen[key(n)]) return false;
      seen[key(n)] = true; return true;
    });
    auRender();
  })();

  /* The give windows, as FireSquire opens them: Buy Me a Coffee's widget,
     PayPal's buttons under the house's own controls, and an address and a QR
     code per network. Escape, the scrim and Close shut a window, the focus
     stays inside while it is open and goes back to the button that opened it. */
  var scrim = wins.querySelector(".sl-scrim");
  var shown = null, opener = null;
  var FOCUSABLE = "a[href], button:not([disabled]), input:not([disabled]), select, textarea, iframe, [tabindex]";
  function focusables(w) {
    return Array.prototype.filter.call(w.querySelectorAll(FOCUSABLE), function (n) { return n.tabIndex >= 0 && n.getClientRects().length > 0; });
  }
  function showWin(name, from) {
    var w = byId("sl-win-" + name);
    hideTip();
    opener = from;
    shown = w;
    scrim.hidden = false;
    w.hidden = false;
    var into = GIVE[name].open(w);
    paint();
    (into || w).focus();
  }
  function closeWin() {
    if (!shown) return;
    var w = shown;
    shown = null;
    w.hidden = true;
    scrim.hidden = true;
    GIVE[w.id.replace("sl-win-", "")].close(w);
    if (opener && document.contains(opener)) opener.focus();
  }
  document.addEventListener("keydown", function (e) {
    if (!shown) return;
    if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); closeWin(); return; }
    if (e.key !== "Tab") return;
    var f = focusables(shown), a = document.activeElement;
    if (!f.length) { e.preventDefault(); shown.focus(); return; }
    var first = f[0], last = f[f.length - 1];
    if (!shown.contains(a)) { e.preventDefault(); (e.shiftKey ? last : first).focus(); }
    else if (e.shiftKey && (a === first || a === shown)) { e.preventDefault(); last.focus(); }
    else if (!e.shiftKey && a === last) { e.preventDefault(); first.focus(); }
  }, true);
  scrim.addEventListener("click", closeWin);
  wins.querySelectorAll("[data-sl-close]").forEach(function (b) { b.addEventListener("click", closeWin); });
  // A plain click opens the window; one that asks for a new tab still reaches
  // the hosted page. The window keeps the donor on the page, so Unraid's
  // warning about leaving for an outside site, delegated from <body>, has
  // nothing to say.
  root.querySelectorAll("[data-sl-give]").forEach(function (a) {
    a.addEventListener("click", function (e) {
      if (e.button || e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) return;
      e.preventDefault();
      e.stopPropagation();
      showWin(a.getAttribute("data-sl-give"), a);
    });
  });
  function shake(node) {
    node.classList.remove("sl-shake");
    void node.offsetWidth;
    node.classList.add("sl-shake");
  }

  var GIVE = {
    coffee: {
      open: function (w) {
        var box = w.querySelector(".sl-frame"), f = document.createElement("iframe");
        f.src = box.getAttribute("data-src");
        f.title = "Buy Me a Coffee";
        f.allow = "payment";
        box.appendChild(f);
        return null;
      },
      close: function (w) { w.querySelector(".sl-frame").textContent = ""; }
    },
    paypal: {
      open: function () {
        pp.freq = "once";
        pp.preset = PP_DEFAULT;
        ppField.value = "";
        ppSegs.often.set(pp.freq);
        ppMark();
        ppRender();
        return ppSegs.often.opts[0];
      },
      close: function () { pp.run++; ppClose(); }
    },
    crypto: {
      open: function () {
        pickCoin(cwin.querySelector(".sl-coin"));
        return copyBtn;
      },
      close: function () { clearTimeout(copiedTimer); }
    }
  };

  /* The house's horizontal selector: one option is picked, the arrow keys move
     the pick, and only the picked option is a tab stop. */
  function selector(node, pick) {
    var opts = Array.prototype.slice.call(node.children);
    var rtl = getComputedStyle(node).direction === "rtl";
    function set(v) {
      var any = false;
      opts.forEach(function (b) {
        var on = b.getAttribute("data-value") === v;
        any = any || on;
        b.setAttribute("aria-checked", on ? "true" : "false");
        b.tabIndex = on ? 0 : -1;
      });
      if (!any) opts[0].tabIndex = 0;
    }
    opts.forEach(function (b, i) {
      b.addEventListener("click", function () { set(b.getAttribute("data-value")); pick(b.getAttribute("data-value")); });
      b.addEventListener("keydown", function (e) {
        var step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
        if (!step) return;
        e.preventDefault();
        if (rtl && (e.key === "ArrowRight" || e.key === "ArrowLeft")) step = -step;
        var next = opts[(i + step + opts.length) % opts.length];
        next.click();
        next.focus();
      });
    });
    return { set: set, opts: opts };
  }

  /* PayPal. A one-off donation and a recurring one need the SDK loaded with
     different parameters, so each gets its own namespace. A recurring one is a
     subscription to a plan priced at 1 EUR, with the amount as its quantity.
     Only the PayPal wallet and the card button stay; the other funding sources
     repeat what the card covers. */
  var PAYPAL = {
    clientId: "BAAbFqgNYfuCIBT_gwVE64oqj-E-jmxFiLaoR1yMIF9KK-CW16x5Pt2bSjBloqbTF4TvjFYw3ZTLnRP8_U",
    plans: { month: "P-2ND5083133959702RNK2375A", year: "P-2FN843952N550243RNK2375A" },
    currency: "EUR"
  };
  var PP_AMOUNTS = ["10", "25", "50"], PP_DEFAULT = "25";
  var PP_OFF = "sepa,paylater,venmo,bancontact,blik,eps,giropay,ideal,mybank,p24,sofort";
  var pp = { freq: "once", preset: PP_DEFAULT, buttons: null, run: 0 }, ppLoads = {};
  var pwin = byId("sl-win-paypal"), ppBox = byId("sl-pp-buttons"), ppField = byId("sl-pp-other"), ppState = byId("sl-pp-state");
  // A typed amount as PayPal's decimal string, or null below 1 or with more
  // than two decimals. A comma counts as the decimal mark.
  function ppParse(text) {
    var v = String(text).replace(",", ".").trim();
    if (!/^\d+(\.\d{1,2})?$/.test(v) || Number(v) < 1) return null;
    return Number(v).toFixed(2).replace(/\.00$/, "");
  }
  function ppAmount() { return ppField.value === "" ? pp.preset : ppParse(ppField.value); }
  var ppSegs = {
    often: selector(pwin.querySelector('[data-sl-seg="often"]'), function (v) {
      if (v === pp.freq) return;
      pp.freq = v;
      ppRender();
    }),
    amount: selector(pwin.querySelector('[data-sl-seg="amount"]'), function (v) {
      pp.preset = v;
      ppField.value = "";
      ppMark();
    })
  };
  function ppMark() {
    ppSegs.amount.set(pp.preset);
    var ok = !!ppParse(ppField.value);
    ppField.classList.toggle("sl-amount-ok", ok);
    ppField.setAttribute("aria-invalid", ppField.value !== "" && !ok ? "true" : "false");
  }
  // A valid typed amount takes the pick from the presets, so the window never
  // shows two amounts; clearing the field gives it back.
  ppField.addEventListener("input", function () {
    if (ppParse(ppField.value)) pp.preset = null;
    else if (ppField.value === "") pp.preset = pp.preset || PP_DEFAULT;
    ppMark();
  });
  // Leaving the field shows the amount as it will be charged, hands a preset's
  // amount back to its preset, and shakes text that is no amount.
  ppField.addEventListener("blur", function () {
    if (ppField.value === "") return;
    var v = ppParse(ppField.value);
    if (!v) { shake(ppField); return; }
    if (PP_AMOUNTS.indexOf(v) >= 0) { pp.preset = v; ppField.value = ""; } else ppField.value = v;
    ppMark();
  });
  function ppSay(cls, text) { ppState.className = "sl-pp-state" + (cls ? " " + cls : ""); ppState.textContent = text; }
  function ppLoad(recurring) {
    var ns = recurring ? "slPaypalRecurring" : "slPaypalOnce";
    if (ppLoads[ns]) return ppLoads[ns];
    var q = "client-id=" + encodeURIComponent(PAYPAL.clientId) + "&currency=" + PAYPAL.currency +
      "&intent=" + (recurring ? "subscription&vault=true" : "capture") + "&components=buttons&disable-funding=" + PP_OFF;
    ppLoads[ns] = new Promise(function (resolve, reject) {
      var s = document.createElement("script");
      s.src = "https://www.paypal.com/sdk/js?" + q;
      s.setAttribute("data-namespace", ns);
      // A blocker can let the script load and still keep the SDK from running.
      s.onload = function () { if (window[ns]) resolve(window[ns]); else reject(new Error("paypal")); };
      s.onerror = function () { delete ppLoads[ns]; s.remove(); reject(new Error("paypal")); };
      document.head.appendChild(s);
    });
    return ppLoads[ns];
  }
  function ppClose() {
    if (pp.buttons) pp.buttons.close().catch(function () {});
    pp.buttons = null;
    ppBox.textContent = "";
  }
  function ppRender() {
    var mine = ++pp.run, freq = pp.freq;
    ppSay("", T.s_give_loading);
    ppClose();
    function thanks() { if (mine === pp.run) ppSay("sl-ok", T.s_give_thanks); }
    function failed() { if (mine === pp.run) ppSay("sl-err", T.s_give_failed); }
    ppLoad(freq !== "once").then(function (paypal) {
      if (mine !== pp.run) return;
      var handlers = freq === "once" ? {
        createOrder: function (d, actions) {
          var money = { currency_code: PAYPAL.currency, value: ppAmount() };
          return actions.order.create({
            purchase_units: [{
              description: "ShipLog",
              amount: { currency_code: money.currency_code, value: money.value, breakdown: { item_total: money } },
              // DONATION marks the payment as a gift in the donor's PayPal history.
              items: [{ name: "ShipLog", quantity: "1", category: "DONATION", unit_amount: money }]
            }]
          });
        },
        onApprove: function (d, actions) { return actions.order.capture().then(thanks); }
      } : {
        createSubscription: function (d, actions) {
          return actions.subscription.create({ plan_id: PAYPAL.plans[freq], quantity: String(Math.max(1, Math.round(Number(ppAmount())))) });
        },
        onApprove: thanks
      };
      pp.buttons = paypal.Buttons(Object.assign({
        style: { layout: "vertical", color: "blue", shape: "rect", borderRadius: 10, label: "donate", height: 40 },
        // A half-typed amount keeps the buttons from opening PayPal at all.
        onClick: function (d, actions) { return ppAmount() ? actions.resolve() : actions.reject(); },
        onError: failed
      }, handlers));
      return pp.buttons.render(ppBox).then(function () { if (mine === pp.run) ppSay("", ""); });
    }).catch(failed);
  }

  /* Crypto. The code comes first, since it is what the window is opened for;
     the chain is switched right under the address it changes, and the coins
     wait below as tiles. A new coin starts on its own first chain, never on one
     carried over. */
  var cwin = byId("sl-win-crypto"), copyBtn = byId("sl-copy"), note = cwin.querySelector(".sl-note"), copiedTimer = 0;
  function shownAddress() {
    return Array.prototype.filter.call(cwin.querySelectorAll(".sl-qrpanel"), function (p) { return !p.hidden; })[0].getAttribute("data-address");
  }
  function pickNet(chip) {
    Array.prototype.forEach.call(chip.parentNode.children, function (c) { c.setAttribute("aria-selected", c === chip ? "true" : "false"); });
    cwin.querySelectorAll(".sl-qrpanel").forEach(function (p) { p.hidden = p.getAttribute("data-address") !== chip.getAttribute("data-address"); });
    note.hidden = !chip.hasAttribute("data-note");
    clearTimeout(copiedTimer);
    copyBtn.textContent = T.s_copy;
  }
  function pickCoin(tile) {
    var id = tile.getAttribute("data-coin");
    cwin.querySelectorAll(".sl-coin").forEach(function (t) { t.setAttribute("aria-selected", t === tile ? "true" : "false"); });
    cwin.querySelectorAll(".sl-nets").forEach(function (r) { r.hidden = r.getAttribute("data-coin") !== id; });
    pickNet(cwin.querySelector('.sl-nets[data-coin="' + id + '"] .sl-net'));
    paint();
  }
  cwin.querySelectorAll(".sl-coin").forEach(function (t) { t.addEventListener("click", function () { pickCoin(t); }); });
  cwin.querySelectorAll(".sl-net").forEach(function (c) { c.addEventListener("click", function () { pickNet(c); c.focus(); }); });
  // Without HTTPS there is no clipboard API, and a browser may refuse it; a
  // hidden field copied the old way stands in for it.
  function copyText(text) {
    if (navigator.clipboard && window.isSecureContext) return navigator.clipboard.writeText(text).catch(function () { return copyOld(text); });
    return copyOld(text);
  }
  function copyOld(text) {
    return new Promise(function (resolve, reject) {
      var ta = document.createElement("textarea");
      ta.className = "sl-copy-src";
      ta.value = text;
      ta.setAttribute("readonly", "");
      cwin.appendChild(ta);
      ta.select();
      var ok = false;
      try { ok = document.execCommand("copy"); } catch (e) {}
      ta.remove();
      if (ok) resolve(); else reject(new Error("copy"));
    });
  }
  copyBtn.addEventListener("click", function () {
    copyText(shownAddress()).then(function () {
      copyBtn.textContent = T.s_copied;
      clearTimeout(copiedTimer);
      copiedTimer = setTimeout(function () { copyBtn.textContent = T.s_copy; }, 1500);
    }, function () { shake(copyBtn); });
  });

  auShown();
  engines();
  // TowerTorch sets the accent from a deferred script.
  window.addEventListener("load", engines);
  if (document.fonts && document.fonts.ready) document.fonts.ready.then(fit);
})();
