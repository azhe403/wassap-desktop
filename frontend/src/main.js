import './style.css';
import './app.css';

// DOM elements
const app = document.querySelector('#app');

app.innerHTML = `
<div class="container">
    <header class="header">
        <h1>Wassap Desktop — Pecut 2</h1>
        <p class="subtitle">Multi-Account WebView2 Profile Isolation Proof of Concept</p>
    </header>

    <div class="current-bar">
        <span class="label">Current Profile:</span>
        <span id="current-profile" class="profile-badge none">None</span>
        <span id="status-indicator" class="status-indicator ready">Ready</span>
    </div>

    <div class="actions-card">
        <h2>Actions</h2>
        <div class="button-group">
            <button id="btn-open-a" class="btn btn-primary">
                <span class="icon">📱</span> Open Account A
            </button>
            <button id="btn-open-b" class="btn btn-secondary">
                <span class="icon">📱</span> Open Account B
            </button>
            <button id="btn-close-current" class="btn btn-danger">
                <span class="icon">⏹</span> Close Current WebView
            </button>
            <button id="btn-refresh" class="btn btn-ghost">
                <span class="icon">🔄</span> Refresh Status
            </button>
        </div>
    </div>

    <div class="profiles-card">
        <h2>Profile Storage & Isolation Status</h2>
        <div id="profiles-list" class="profiles-grid">
            <div class="loading">Loading profiles...</div>
        </div>
    </div>

    <div class="accounts-card">
        <h2>Account Management</h2>
        <p class="hint">Create, rename, delete accounts. Each account gets its own persistent browser profile.</p>
        <div class="account-form">
            <input id="account-name" class="account-input" placeholder="New account name" maxlength="100">
            <button id="btn-create-account" class="btn btn-sm btn-primary">+ Create</button>
        </div>
        <div id="accounts-list" class="accounts-list">
            <div class="loading">Loading accounts...</div>
        </div>
        <p id="accounts-msg" class="hint"></p>
    </div>

    <div class="evidence-card">
        <h2>Evidence Inspector</h2>
        <p class="hint">Select an account to view real on-disk storage data (cookies, LocalStorage leveldb, paths).</p>
        <div class="inspector-buttons">
            <button id="btn-inspect-a" class="btn btn-sm">Inspect Account A</button>
            <button id="btn-inspect-b" class="btn btn-sm">Inspect Account B</button>
        </div>
        <div id="evidence-output" class="evidence-output">
            <p class="placeholder">Click an inspect button above to load on-disk evidence.</p>
        </div>
    </div>

    <div class="legacy-card">
        <h3>Pecut 1 Compatibility</h3>
        <p>Launch the single-session WhatsApp Web instance using the original Pecut 1 profile path (~/.local/share/wassap-desktop):</p>
        <button id="btn-legacy" class="btn btn-outline">Launch Pecut 1 Legacy Session</button>
    </div>

    <footer class="footer">
        <p>Both accounts navigate to: <code>https://web.whatsapp.com</code> | Isolated User Data Folders</p>
    </footer>
</div>
`;

// Wire up Wails backend methods via window.go.main.App
const getApp = () => window.go && window.go.main && window.go.main.App;

async function updateStatus() {
    const api = getApp();
    if (!api) {
        document.getElementById('current-profile').innerText = 'Backend loading...';
        return;
    }

    try {
        const current = await api.CurrentProfile();
        const badge = document.getElementById('current-profile');
        if (current && current.length > 0) {
            badge.innerText = current;
            badge.className = 'profile-badge active';
        } else {
            badge.innerText = 'None';
            badge.className = 'profile-badge none';
        }

        const profiles = await api.ListProfiles();
        renderProfiles(profiles);
        await refreshAccounts();
    } catch (err) {
        console.error('Failed to update status:', err);
    }
}

function renderProfiles(profiles) {
    const container = document.getElementById('profiles-list');
    if (!profiles || profiles.length === 0) {
        container.innerHTML = '<p class="empty">No profiles found.</p>';
        return;
    }

    container.innerHTML = profiles.map(p => `
        <div class="profile-card ${p.open ? 'open' : 'closed'}">
            <div class="profile-header">
                <h3>${p.id}</h3>
                <span class="badge ${p.open ? 'badge-open' : 'badge-closed'}">
                    ${p.open ? 'OPEN' : 'CLOSED'}
                </span>
            </div>
            <div class="profile-details">
                <p><strong>Path:</strong> <code>${escapeHTML(p.path)}</code></p>
                <p><strong>Storage Persisted:</strong> ${p.storage.persisted ? '✅ Yes' : '⚪ Not yet'}</p>
                <p><strong>Storage Size:</strong> ${formatBytes(p.storage.bytes)}</p>
                <p><strong>Cookies Hosts:</strong> ${p.storage.cookies >= 0 ? p.storage.cookies + ' hosts' : 'None / locked'}</p>
            </div>
            <div class="profile-card-actions">
                ${p.open ? 
                    `<button class="btn btn-xs btn-danger" onclick="closeProfile('${p.id}')">Close</button>` : 
                    `<button class="btn btn-xs btn-primary" onclick="openProfile('${p.id}')">Open</button>`
                }
                <button class="btn btn-xs btn-ghost" onclick="inspectProfile('${p.id}')">Inspect</button>
            </div>
        </div>
    `).join('');
}

async function inspectProfile(id) {
    const api = getApp();
    if (!api) return;
    const output = document.getElementById('evidence-output');
    output.innerHTML = '<div class="loading">Reading storage evidence...</div>';

    try {
        const items = await api.InspectProfile(id);
        if (!items || items.length === 0) {
            output.innerHTML = `<p class="empty">No evidence items recorded for ${id}.</p>`;
            return;
        }

        output.innerHTML = `
            <h4>Evidence for Profile: <code>${id}</code></h4>
            <table class="evidence-table">
                <thead>
                    <tr>
                        <th>Check</th>
                        <th>Label</th>
                        <th>Observed Value</th>
                    </tr>
                </thead>
                <tbody>
                    ${items.map(it => `
                        <tr class="row-${it.Relevance}">
                            <td><code>${it.Key}</code></td>
                            <td>${it.Label}</td>
                            <td><code>${escapeHTML(it.Value)}</code></td>
                        </tr>
                    `).join('')}
                </tbody>
            </table>
        `;
    } catch (err) {
        output.innerHTML = `<p class="error">Failed to inspect ${id}: ${err}</p>`;
    }
}

// --- Account management (Pecut 3) ---
async function refreshAccounts() {
    const api = getApp();
    const list = document.getElementById('accounts-list');
    const msg = document.getElementById('accounts-msg');
    if (!api) {
        list.innerHTML = '<div class="loading">Backend loading...</div>';
        return;
    }
    msg.innerText = '';
    let accounts;
    try {
        accounts = await api.ListAccounts();
    } catch (err) {
        msg.innerText = 'List failed: ' + err;
        return;
    }
    if (!accounts || accounts.length === 0) {
        list.innerHTML = '<p class="empty">No accounts yet.</p>';
        return;
    }
    list.innerHTML = accounts.map(a => `
        <div class="account-row">
            <div class="account-info">
                <strong>${escapeHTML(a.Name)}</strong>
                <code>${a.ID}</code>
            </div>
            <div class="profile-card-actions">
                <button class="btn btn-xs btn-ghost" onclick="renameAccount('${a.ID}')">Rename</button>
                <button class="btn btn-xs btn-danger" onclick="deleteAccount('${a.ID}')">Delete</button>
            </div>
        </div>
    `).join('');
}

window.renameAccount = async (id) => {
    const api = getApp();
    if (!api) return;
    let name;
    try {
        name = await api.GetAccount(id);
        name = prompt('New account name:', name ? name.Name : '');
    } catch (err) {
        alert('Failed to load account: ' + err);
        return;
    }
    if (!name) return;
    try {
        await api.RenameAccount(id, name);
        await refreshAccounts();
    } catch (err) {
        alert('Rename failed: ' + err);
    }
};

window.deleteAccount = async (id) => {
    const api = getApp();
    if (!api) return;
    if (!confirm('Delete account ' + id + ' and its browser profile?')) return;
    try {
        await api.RemoveAccount(id);
        await refreshAccounts();
        await updateStatus();
    } catch (err) {
        alert('Delete failed: ' + err);
    }
};

// Global actions for inline buttons
window.openProfile = async (id) => {
    const api = getApp();
    if (!api) return;
    setStatus('Opening ' + id + '...');
    try {
        await api.OpenAccount(id);
        setStatus('Ready');
        await updateStatus();
    } catch (err) {
        alert('Failed to open profile: ' + err);
        setStatus('Error');
    }
};

window.closeProfile = async (id) => {
    const api = getApp();
    if (!api) return;
    setStatus('Closing ' + id + '...');
    try {
        await api.CloseAccount(id);
        setStatus('Ready');
        await updateStatus();
    } catch (err) {
        alert('Failed to close profile: ' + err);
        setStatus('Error');
    }
};

window.inspectProfile = inspectProfile;

function setStatus(text) {
    const el = document.getElementById('status-indicator');
    if (el) el.innerText = text;
}

function escapeHTML(str) {
    if (!str) return '';
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function formatBytes(bytes) {
    if (!bytes || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB'];
    let i = 0;
    let b = bytes;
    while (b >= 1024 && i < units.length - 1) {
        b /= 1024;
        i++;
    }
    return b.toFixed(1) + ' ' + units[i];
}

// Event listeners
document.getElementById('btn-open-a').addEventListener('click', () => window.openProfile('account-a'));
document.getElementById('btn-open-b').addEventListener('click', () => window.openProfile('account-b'));
document.getElementById('btn-close-current').addEventListener('click', async () => {
    const api = getApp();
    if (!api) return;
    setStatus('Closing current WebView...');
    try {
        await api.CloseCurrent();
        setStatus('Ready');
        await updateStatus();
    } catch (err) {
        alert('Failed to close: ' + err);
    }
});
document.getElementById('btn-refresh').addEventListener('click', updateStatus);
document.getElementById('btn-create-account').addEventListener('click', async () => {
    const api = getApp();
    if (!api) return;
    const input = document.getElementById('account-name');
    const name = input.value.trim();
    if (!name) return;
    input.value = '';
    try {
        const acc = await api.CreateAccount(name);
        await refreshAccounts();
        await updateStatus();
        setStatus('Created ' + acc.Name);
    } catch (err) {
        alert('Create failed: ' + err);
    }
});
document.getElementById('btn-inspect-a').addEventListener('click', () => inspectProfile('account-a'));
document.getElementById('btn-inspect-b').addEventListener('click', () => inspectProfile('account-b'));
document.getElementById('btn-legacy').addEventListener('click', async () => {
    const api = getApp();
    if (!api) return;
    try {
        await api.OpenLegacyPecut1();
        await updateStatus();
    } catch (err) {
        alert('Failed to open legacy session: ' + err);
    }
});

// Periodic polling
setInterval(updateStatus, 3000);
window.addEventListener('DOMContentLoaded', updateStatus);
setTimeout(() => { updateStatus(); refreshAccounts(); }, 500);
