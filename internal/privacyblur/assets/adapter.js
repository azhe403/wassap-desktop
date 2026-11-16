/* Wassap Desktop adapter: bridges the vendored engine's chrome.* API to the
   app's config store. Never replace window.chrome — WebView2 owns
   chrome.webview. Augment only. */
(function () {
  'use strict';
  var boot = window.__WA_PRIVACY_BOOT || {};
  var changedListeners = [];

  function fireChanged(changes) {
    for (var i = 0; i < changedListeners.length; i++) {
      try { changedListeners[i](changes, 'local'); } catch (e) { /* an engine hook must not break us */ }
    }
  }

  var local = {
    get: function (defaults, cb) {
      // Chrome accepts string, array or object defaults.
      var keys;
      if (typeof defaults === 'string') {
        keys = [defaults];
      } else if (Array.isArray(defaults)) {
        keys = defaults.slice();
      } else if (defaults && typeof defaults === 'object') {
        keys = Object.keys(defaults);
      } else {
        keys = [];
      }
      var out = {};
      for (var i = 0; i < keys.length; i++) {
        var has = Object.prototype.hasOwnProperty.call(boot, keys[i]);
        if (has) {
          out[keys[i]] = boot[keys[i]];
        } else if (defaults && !Array.isArray(defaults) && typeof defaults === 'object') {
          out[keys[i]] = defaults[keys[i]];
        } else {
          out[keys[i]] = undefined;
        }
      }
      if (typeof cb === 'function') cb(out);
    },
    set: function (obj, cb) {
      var changes = {};
      if (obj && typeof obj === 'object') {
        for (var k in obj) {
          if (Object.prototype.hasOwnProperty.call(obj, k)) {
            boot[k] = obj[k];
            // oldValue is intentionally undefined: the engine re-gets the whole
            // object on change, so the previous values are never read.
            changes[k] = { oldValue: undefined, newValue: obj[k] };
          }
        }
        try {
          window.chrome.webview.postMessage(JSON.stringify({ kind: 'privacy-blur-set', values: obj }));
        } catch (e) { /* host routing is best-effort */ }
        fireChanged(changes);
      }
      if (typeof cb === 'function') cb();
    }
  };

  window.chrome = window.chrome || {};
  window.chrome.storage = window.chrome.storage || {};
  window.chrome.storage.local = local;
  window.chrome.storage.onChanged = {
    addListener: function (fn) { if (typeof fn === 'function') changedListeners.push(fn); }
  };
  window.chrome.runtime = window.chrome.runtime || {};
  if (!window.chrome.runtime.onMessage) {
    window.chrome.runtime.onMessage = { addListener: function () {} };
  }

  window.__wassapPrivacyApply = function (json) {
    try {
      var next = (typeof json === 'string') ? JSON.parse(json) : json;
      if (!next || typeof next !== 'object') return;
      for (var k in next) {
        if (Object.prototype.hasOwnProperty.call(next, k)) boot[k] = next[k];
      }
      if (typeof window.applySettings === 'function') {
        window.applySettings(Object.assign({}, boot));
      }
    } catch (e) { /* never break the page */ }
  };
})();
