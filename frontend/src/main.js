// This placeholder is never shown in normal operation. Wails navigates to
// http://wails.localhost/ on startup, but no assets are served there: the Go
// asset-server handler redirects every request to WhatsApp Web before this
// module is reached. It exists only so the template's build pipeline keeps a
// valid entry point, and so the window still shows something recognisable if
// that redirect ever fails.
import './style.css';

document.querySelector('#app').innerHTML = `
    <div class="result">
      <p>Wassap Desktop</p>
      <p>Opening WhatsApp Web…</p>
    </div>
`;
