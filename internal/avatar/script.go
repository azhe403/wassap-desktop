package avatar

import "strconv"

// DocumentScript returns the document-created script that watches the
// WhatsApp Web "me" photo and posts data URIs to the Go host. Empty data URIs
// are signalled with an avatar-clear message instead.
func DocumentScript(accountID string) string {
	return `(function () {
  if (window.__wassapAvatarSync) return;
  window.__wassapAvatarSync = true;
  var ACCOUNT = ` + strconv.Quote(accountID) + `;
  var lastUrl = '';
  var debounce;

  function post(kind, dataUrl) {
    try {
      var msg = { kind: kind, account: ACCOUNT };
      if (dataUrl) { msg.values = { dataUrl: dataUrl }; }
      window.chrome.webview.postMessage(JSON.stringify(msg));
    } catch (e) {}
  }

  function selfImage() {
    var row = document.querySelector('[data-testid="message-yourself-row"]');
    var el = row && (row.querySelector('img') || row.querySelector('svg image'));
    if (!el) {
      el = document.querySelector('[data-testid="navbar-item-me-tab-photo"] img') ||
           document.querySelector('[data-testid="navbar-item-me-tab-photo"] svg image');
    }
    return el || null;
  }

  function photoUrl() {
    var el = selfImage();
    if (!el) return '';
    var nw = el.naturalWidth || 0;
    if (nw && nw < 32) return '';
    var src = el.tagName === 'IMG'
      ? (el.currentSrc || el.src || '')
      : (el.getAttribute('href') || el.getAttributeNS('http://www.w3.org/1999/xlink', 'href') || '');
    if (!src) return '';
    var host = '';
    try { host = new URL(src, location.href).hostname; } catch (e) { return ''; }
    // Only media-host photos count. This excludes static.whatsapp.net chrome
    // artwork and data:/blob: placeholder images.
    if (!/(^|\.)cdn\.whatsapp\.net$/.test(host) && host !== 'pps.whatsapp.net') return '';
    return src;
  }

  function sendUrl(url) {
    try {
      fetch(url).then(function (r) { return r.blob(); }).then(function (blob) {
        return new Promise(function (resolve, reject) {
          var reader = new FileReader();
          reader.onload = function () { resolve(String(reader.result || '')); };
          reader.onerror = reject;
          reader.readAsDataURL(blob);
        });
      }).then(function (dataUrl) {
        if (dataUrl && dataUrl.indexOf('data:image/') === 0) post('avatar-update', dataUrl);
      }).catch(function () {});
    } catch (e) {}
  }

  function scan() {
    try {
      var url = photoUrl();
      if (lastUrl && !url) {
        lastUrl = '';
        post('avatar-clear');
        return;
      }
      if (url && lastUrl !== url) {
        lastUrl = url;
        sendUrl(url);
      }
    } catch (e) {}
  }

  try {
    if (document.body && window.MutationObserver) {
      var observer = new MutationObserver(function () {
        clearTimeout(debounce);
        debounce = setTimeout(scan, 2000);
      });
      observer.observe(document.body, { childList: true, subtree: true, attributes: true });
    }
  } catch (e) {}
  setTimeout(scan, 2500);
  setInterval(scan, 30000);
})();`
}
