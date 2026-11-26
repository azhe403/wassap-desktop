import { getInitials, getAvatarColor } from '../utils/avatar.js';
import { avatarImageHTML } from '../utils/avatar-image.js';

/**
 * Selected account survives re-renders within the session, mirroring how the
 * Settings page keeps its active panel.
 */
let selectedProfileId = null;

/**
 * Last samples seen by the page, kept so a re-paint (selecting another
 * account) shows real numbers immediately instead of placeholders.
 */
let lastUsage = null;
const diskBytesByAccount = new Map();
const storageInfoByAccount = new Map();

/**
 * Renders the full-page Profile Management view inside #content-view.
 * Layout mirrors the Settings page: a fixed header, an account list on the
 * left, and a scrollable detail panel on the right.
 *
 * @param {HTMLElement} container
 * @param {Array} accounts - [{ id, name, createdAt }]
 * @param {string|null} activeId - the account whose session is on screen
 * @param {Object} handlers - { onRename, onRemove }
 * @param {Object} avatars - account id -> profile photo data URI
 */
export function renderProfilesPage(container, accounts, activeId, handlers = {}, avatars = {}) {
    const list = Array.isArray(accounts) ? accounts : [];

    if (list.length === 0) {
        selectedProfileId = null;
    } else if (!list.some(acc => acc.id === selectedProfileId)) {
        // Prefer the account currently on screen, otherwise the first one.
        selectedProfileId = list.some(acc => acc.id === activeId) ? activeId : list[0].id;
    }

    const paint = () => {
        const selected = list.find(acc => acc.id === selectedProfileId) || null;

        container.innerHTML = `
            <div class="settings-page-view profiles-page-view" role="region" aria-label="Profile Management">
                <div class="settings-page-header">
                    <h2 class="settings-page-title">Profile Management</h2>
                    <p class="settings-page-subtitle">Rename, remove and inspect your account profiles. Each account keeps its own isolated browser storage.</p>
                </div>

                <div class="profiles-total-usage" id="profiles-total-usage">Reading resource usage…</div>

                ${list.length === 0 ? `
                    <p class="profiles-empty">No accounts yet. Add one with the ➕ button in the sidebar.</p>
                ` : `
                    <div class="settings-shell profiles-shell">
                        <nav class="settings-nav profiles-nav" aria-label="Accounts">
                            ${list.map(acc => renderNavItem(acc, activeId, avatars)).join('')}
                        </nav>
                        <div class="settings-panels profiles-panels">
                            ${selected ? renderDetail(selected, activeId, avatars) : ''}
                        </div>
                    </div>
                `}
            </div>
        `;

        // Account list: selecting one swaps the detail panel.
        container.querySelectorAll('.profile-nav-item').forEach(btn => {
            btn.addEventListener('click', () => {
                selectedProfileId = btn.getAttribute('data-id');
                paint();
            });
        });

        const card = container.querySelector('.profile-mgmt-card');
        if (card && selected) {
            const renameBtn = card.querySelector('.btn-rename');
            if (renameBtn && handlers.onRename) {
                renameBtn.addEventListener('click', () => handlers.onRename(selected));
            }

            const removeBtn = card.querySelector('.btn-remove');
            if (removeBtn && handlers.onRemove) {
                removeBtn.addEventListener('click', () => handlers.onRemove(selected));
            }

            const inspectBtn = card.querySelector('.btn-inspect');
            if (inspectBtn) {
                inspectBtn.addEventListener('click', () => toggleInspect(card, selected.id));
            }
        }

        // Fill in storage stats asynchronously; ListProfiles walks each profile's
        // user data folder, so it must not block the first paint.
        loadStorageSummaries(container, list);

        // Re-paints rebuild the markup; put the last sample back immediately so
        // the page never flashes "Reading…" after a click.
        applyUsage();
        applyStorage();
    };

    paint();
}

function renderNavItem(acc, activeId, avatars = {}) {
    const color = getAvatarColor(acc.id || acc.name);
    const initials = getInitials(acc.name);
    const isSelected = acc.id === selectedProfileId;
    const isActive = acc.id === activeId;

    return `
        <button
            type="button"
            class="settings-nav-item profile-nav-item ${isSelected ? 'active' : ''}"
            data-id="${escapeAttr(acc.id)}"
            aria-current="${isSelected ? 'true' : 'false'}"
        >
            <span class="profile-nav-avatar" style="background-color: ${color.bg}; color: ${color.fg};">
                <span>${escapeHTML(initials)}</span>
                ${avatarImageHTML(acc.id, avatars, '', 'account-avatar-static-img')}
            </span>
            <span class="profile-nav-text">
                <span class="profile-nav-name">${escapeHTML(acc.name)}</span>
                <span class="profile-nav-metrics" data-nav-metrics-for="${escapeAttr(acc.id)}">${navMetricsHTML(acc.id)}</span>
            </span>
            ${isActive ? '<span class="profile-nav-dot" title="Currently open"></span>' : ''}
        </button>
    `;
}

/** Total persisted profile storage, from the latest ListProfiles sample. */
function totalDiskLabel() {
    if (diskBytesByAccount.size === 0) return '…';
    let total = 0;
    for (const bytes of diskBytesByAccount.values()) total += bytes;
    return formatUsageBytes(total);
}

/** Compact CPU / RAM / disk line for one account, from the cached samples. */
function navMetricsHTML(accountId) {
    const profile = ((lastUsage && lastUsage.profiles) || []).find(p => p.id === accountId);
    const disk = diskBytesByAccount.get(accountId);

    const ram = profile && profile.running ? formatUsageBytes(profile.workingSetBytes) : '—';
    const cpu = profile && profile.running ? `${profile.cpuPercent.toFixed(1)}%` : '—';
    const diskLabel = disk === undefined ? '…' : formatBytes(disk);

    return `
        <span class="nav-metric" title="CPU in use"><span class="nav-metric-label">CPU</span>${escapeHTML(cpu)}</span>
        <span class="nav-metric" title="Memory in use"><span class="nav-metric-label">RAM</span>${escapeHTML(ram)}</span>
        <span class="nav-metric" title="Profile size on disk"><span class="nav-metric-label">Disk</span>${escapeHTML(diskLabel)}</span>
    `;
}

function renderDetail(acc, activeId, avatars = {}) {
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
                </div>
            </div>

            <div class="profile-mgmt-facts">
                <div class="profile-mgmt-storage" data-storage-for="${escapeAttr(acc.id)}">Reading storage…</div>
                <div class="profile-mgmt-resources" data-resources-for="${escapeAttr(acc.id)}">Reading usage…</div>
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

async function loadStorageSummaries(container, accounts) {
    const api = window.go && window.go.app && window.go.app.App;
    if (!api || !api.ListProfiles) return;

    try {
        const profiles = await api.ListProfiles();
        diskBytesByAccount.clear();
        storageInfoByAccount.clear();
        (profiles || []).forEach(p => {
            const id = p.id || p.ID;
            storageInfoByAccount.set(id, p);
            const storage = p.storage || {};
            const bytes = storage.persisted ? (storage.bytes || 0) : 0;
            diskBytesByAccount.set(id, bytes);
        });

        applyStorage();
    } catch (err) {
        console.error('Failed to load profile storage info:', err);
        container.querySelectorAll('.profile-mgmt-storage').forEach(el => {
            el.textContent = 'Storage info unavailable';
        });
    }
}

/**
 * Writes the cached storage summary into the current DOM. Like applyUsage,
 * this runs at the end of every paint so switching accounts does not flash the
 * "Reading storage…" placeholder.
 */
function applyStorage() {
    document.querySelectorAll('.profile-mgmt-storage').forEach(el => {
        const id = el.getAttribute('data-storage-for');
        if (!storageInfoByAccount.has(id)) return;
        el.innerHTML = renderStorage(storageInfoByAccount.get(id));
    });

    document.querySelectorAll('.profile-nav-metrics').forEach(el => {
        const id = el.getAttribute('data-nav-metrics-for');
        if (!diskBytesByAccount.has(id)) return;
        el.innerHTML = navMetricsHTML(id);
    });

    applyTotalUsage();
}

function applyTotalUsage() {
    const usage = lastUsage;
    const totalEl = document.getElementById('profiles-total-usage');
    if (!totalEl || !usage) return;
    const ram = formatUsageBytes(usage.totalWorkingSetBytes);
    const cpu = usage.totalCPUPercent.toFixed(1);
    const disk = totalDiskLabel();
    totalEl.innerHTML = `Wassap Desktop total&nbsp; <strong>${ram} RAM</strong> · <strong>${cpu}% CPU</strong> · <strong>${escapeHTML(disk)} DISK</strong>`;
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
    lastUsage = usage;
    applyUsage();
}

/**
 * Writes the cached sample into whatever DOM is currently on screen. Every
 * re-paint (selecting another account) rebuilds the markup, so this runs at the
 * end of each paint as well: without it the totals and the detail line would
 * fall back to their placeholders until the next poll tick.
 */
function applyUsage() {
    const usage = lastUsage;
    if (!usage) return;

    applyTotalUsage();

    // Compact CPU / RAM / disk line under every account in the left-hand list.
    document.querySelectorAll('.profile-nav-metrics').forEach(el => {
        el.innerHTML = navMetricsHTML(el.getAttribute('data-nav-metrics-for'));
    });

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
