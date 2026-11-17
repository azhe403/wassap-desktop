import { getInitials, getAvatarColor } from '../utils/avatar.js';
import { avatarImageHTML } from '../utils/avatar-image.js';

/**
 * Renders the full-page Profile Management view inside #content-view.
 * Lists every account with its on-disk profile storage and per-account
 * Rename / Remove / Inspect actions.
 *
 * @param {HTMLElement} container
 * @param {Array} accounts - [{ id, name, createdAt }]
 * @param {string|null} activeId - the account whose session is on screen
 * @param {Object} handlers - { onRename, onRemove }
 * @param {Object} avatars - account id -> profile photo data URI
 */
export function renderProfilesPage(container, accounts, activeId, handlers = {}, avatars = {}) {
    container.innerHTML = `
        <div class="settings-page-view profiles-page-view" role="region" aria-label="Profile Management">
            <div class="settings-page-header">
                <h2 class="settings-page-title">Profile Management</h2>
                <p class="settings-page-subtitle">Rename, remove and inspect your account profiles. Each account keeps its own isolated browser storage.</p>
            </div>
            <div class="profiles-total-usage" id="profiles-total-usage">Reading resource usage…</div>

            ${accounts.length === 0 ? `
                <p class="profiles-empty">No accounts yet. Add one with the ➕ button in the sidebar.</p>
            ` : `
                <div class="profiles-list">
                    ${accounts.map(acc => renderCard(acc, activeId, avatars)).join('')}
                </div>
            `}
        </div>
    `;

    // Wire per-card actions
    accounts.forEach(acc => {
        const card = container.querySelector(`.profile-mgmt-card[data-id="${escapeAttr(acc.id)}"]`);
        if (!card) return;

        const renameBtn = card.querySelector('.btn-rename');
        if (renameBtn && handlers.onRename) {
            renameBtn.addEventListener('click', () => handlers.onRename(acc));
        }

        const removeBtn = card.querySelector('.btn-remove');
        if (removeBtn && handlers.onRemove) {
            removeBtn.addEventListener('click', () => handlers.onRemove(acc));
        }

        const inspectBtn = card.querySelector('.btn-inspect');
        if (inspectBtn) {
            inspectBtn.addEventListener('click', () => toggleInspect(card, acc.id));
        }
    });

    // Fill in storage stats asynchronously; ListProfiles walks each profile's
    // user data folder, so it must not block the first paint.
    loadStorageSummaries(container);
}

function renderCard(acc, activeId, avatars = {}) {
    const color = getAvatarColor(acc.id || acc.name);
    const initials = getInitials(acc.name);
    const isActive = acc.id === activeId;
    const created = acc.createdAt ? new Date(acc.createdAt) : null;
    const createdLabel = created && !isNaN(created) ? created.toLocaleDateString() : '—';

    return `
        <div class="profile-mgmt-card ${isActive ? 'active' : ''}" data-id="${escapeAttr(acc.id)}">
            <div class="profile-mgmt-main">
                <div class="account-avatar-static" style="background-color: ${color.bg}; color: ${color.fg};">
                    <span>${escapeHTML(initials)}</span>
                    ${avatarImageHTML(acc.id, avatars, '', 'account-avatar-static-img')}
                </div>
                <div class="profile-mgmt-info">
                    <div class="profile-mgmt-name-row">
                        <span class="profile-mgmt-name">${escapeHTML(acc.name)}</span>
                        ${isActive ? '<span class="profile-state-badge open">Active</span>' : ''}
                    </div>
                    <div class="profile-mgmt-meta">Created ${escapeHTML(createdLabel)}</div>
                    <div class="profile-mgmt-storage" data-storage-for="${escapeAttr(acc.id)}">Reading storage…</div>
                    <div class="profile-mgmt-resources" data-resources-for="${escapeAttr(acc.id)}">Reading usage…</div>
                </div>
            </div>
            <div class="profile-mgmt-actions">
                <button type="button" class="btn-secondary btn-small btn-rename">Rename</button>
                <button type="button" class="btn-secondary btn-small btn-inspect">Inspect</button>
                <button type="button" class="btn-danger btn-small btn-remove" ${isActive ? 'disabled title="Switch to another account first"' : ''}>Remove</button>
            </div>
            <div class="profile-mgmt-evidence" hidden></div>
        </div>
    `;
}

async function loadStorageSummaries(container) {
    const api = window.go && window.go.app && window.go.app.App;
    if (!api || !api.ListProfiles) return;

    try {
        const profiles = await api.ListProfiles();
        const byId = new Map((profiles || []).map(p => [p.id || p.ID, p]));

        container.querySelectorAll('.profile-mgmt-storage').forEach(el => {
            const id = el.getAttribute('data-storage-for');
            const info = byId.get(id);
            el.innerHTML = renderStorage(info);
        });
    } catch (err) {
        console.error('Failed to load profile storage info:', err);
        container.querySelectorAll('.profile-mgmt-storage').forEach(el => {
            el.textContent = 'Storage info unavailable';
        });
    }
}

function renderStorage(info) {
    if (!info) {
        return '<span class="profile-storage-missing">Profile directory not found on disk</span>';
    }
    const stateNum = typeof info.state === 'number' ? info.state : String(info.state ?? '').toLowerCase();
    const stateMap = { 0: 'closed', 1: 'open', 2: 'closing', closed: 'closed', open: 'open', closing: 'closing' };
    const state = stateMap[stateNum] || 'unknown';

    const storage = info.storage || {};
    const parts = [];
    parts.push(`<span class="profile-state-badge ${state === 'open' ? 'open' : 'closed'}">${escapeHTML(state)}</span>`);

    if (storage.persisted) {
        parts.push(`${formatBytes(storage.bytes || 0)} stored`);
        parts.push(storage.cookies >= 0 ? `${storage.cookies} cookie host${storage.cookies === 1 ? '' : 's'}` : 'cookies unreadable');
        if (storage.partial) {
            parts.push('<span class="profile-partial-flag">partial write</span>');
        }
    } else {
        parts.push('no stored session yet');
    }
    return parts.join('<span class="profile-storage-sep">·</span>');
}

async function toggleInspect(card, accountId) {
    const panel = card.querySelector('.profile-mgmt-evidence');
    if (!panel) return;

    if (!panel.hidden) {
        panel.hidden = true;
        return;
    }

    panel.hidden = false;
    if (panel.dataset.loaded === 'true') return;

    panel.innerHTML = '<p class="evidence-loading">Reading profile evidence…</p>';

    const api = window.go && window.go.app && window.go.app.App;
    if (!api || !api.InspectProfile) {
        panel.innerHTML = '<p class="evidence-loading">Inspection is not available.</p>';
        return;
    }

    try {
        const evidence = await api.InspectProfile(accountId);
        if (!evidence || evidence.length === 0) {
            panel.innerHTML = '<p class="evidence-loading">No evidence recorded for this profile yet.</p>';
        } else {
            panel.innerHTML = `
                <table class="evidence-rows">
                    <thead><tr><th>Evidence</th><th>Value</th></tr></thead>
                    <tbody>
                        ${evidence.map(row => `
                            <tr class="evidence-${String(row.relevance || '').toLowerCase()}">
                                <td>${escapeHTML(row.label || row.key || '')}</td>
                                <td>${escapeHTML(row.value || '')}</td>
                            </tr>
                        `).join('')}
                    </tbody>
                </table>
            `;
        }
        panel.dataset.loaded = 'true';
    } catch (err) {
        console.error('Failed to inspect profile:', err);
        panel.innerHTML = `<p class="evidence-loading">Inspection failed: ${escapeHTML(String(err))}</p>`;
    }
}

function formatBytes(bytes) {
    if (!bytes || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB'];
    let value = bytes;
    let unit = 0;
    while (value >= 1024 && unit < units.length - 1) {
        value /= 1024;
        unit += 1;
    }
    return `${value >= 10 || unit === 0 ? Math.round(value) : value.toFixed(1)} ${units[unit]}`;
}

/**
 * Paints a fresh resource sample into the page. Called by the app's 2-second
 * polling loop while the page is open; safe to call with the page closed.
 */
export function updateProfilesUsage(usage) {
    if (!usage) return;

    const totalEl = document.getElementById('profiles-total-usage');
    if (totalEl) {
        totalEl.innerHTML = `Wassap Desktop total&nbsp; <strong>${formatUsageBytes(usage.totalWorkingSetBytes)} RAM</strong> · <strong>${usage.totalCPUPercent.toFixed(1)}% CPU</strong>`;
    }

    const sep = '<span class="profile-storage-sep">·</span>';
    document.querySelectorAll('.profile-mgmt-resources').forEach(el => {
        const id = el.getAttribute('data-resources-for');
        const p = (usage.profiles || []).find(x => x.id === id);
        if (!p || !p.running) {
            el.textContent = 'Not running — no CPU/RAM in use';
            el.classList.add('idle');
            return;
        }
        el.classList.remove('idle');
        const procLabel = `${p.processCount} process${p.processCount === 1 ? '' : 'es'}`;
        el.innerHTML = `<span class="profile-state-badge open">running</span>${sep}${escapeHTML(procLabel)}${sep}${formatUsageBytes(p.workingSetBytes)} RAM${sep}${p.cpuPercent.toFixed(1)}% CPU`;
    });
}

function formatUsageBytes(bytes) {
    if (!bytes || bytes <= 0) return '0 MB';
    const mb = bytes / (1024 * 1024);
    if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
    if (mb >= 1) return `${mb >= 100 ? Math.round(mb) : mb.toFixed(1)} MB`;
    return `${Math.round(bytes / 1024)} KB`;
}

function escapeHTML(str) {
    if (!str) return '';
    return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function escapeAttr(str) {
    if (!str) return '';
    return String(str).replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}
