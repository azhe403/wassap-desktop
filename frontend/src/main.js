import './style.css';
import './app.css';

import { state } from './state.js';
import { renderSidebar, renderEmptyState } from './components/sidebar.js';
import { showAddAccountModal, showRenameModal, showConfirmModal } from './components/modal.js';
import { renderSettingsPage, updateSettingsStatusRow, updatePrivacyBlurControls } from './components/settings-page.js';
import { renderProfilesPage, updateProfilesUsage } from './components/profiles-page.js';
import { fetchAvatars } from './utils/avatar-image.js';

// Access Wails backend bindings
const getApp = () => window.go && window.go.app && window.go.app.App;

// App root containers
const app = document.querySelector('#app');
app.innerHTML = `
    <div id="sidebar-container"></div>
    <main id="content-container" class="content-container">
        <div id="content-view" class="empty-state-view">
            <div class="mini-spinner" style="width: 32px; height: 32px;"></div>
            <p style="margin-top: 16px; color: var(--text-secondary);">Loading Wassap Desktop...</p>
        </div>
    </main>
    <div id="toast-container" class="toast-container" aria-live="polite"></div>
`;

const sidebarContainer = document.querySelector('#sidebar-container');
const contentView = document.querySelector('#content-view');
const toastContainer = document.querySelector('#toast-container');

/**
 * Toast notifications
 */
function showToast(message, type = 'info') {
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.innerText = message;
    toastContainer.appendChild(toast);
    setTimeout(() => {
        if (toast.parentNode) {
            toast.parentNode.removeChild(toast);
        }
    }, 4000);
}

/**
 * Notify the backend about WebView visibility. The native child window must
 * be hidden while a modal or the settings page owns the content area, and only
 * the active account's session is restored afterwards.
 */
function setWebViewVisible(visible) {
    const api = getApp();
    if (api && api.SetWebViewVisible) {
        api.SetWebViewVisible(visible).catch(() => {});
    }
}

/**
 * Views that own the content area and therefore require the native WebView
 * to stay hidden.
 */
function isContentOwningView(view) {
    return view === 'settings' || view === 'profiles';
}

/**
 * Restore the native view for the active account, unless the settings page or
 * a modal currently owns the content area.
 */
function restoreWebView() {
    if (isContentOwningView(state.currentView)) {
        setWebViewVisible(false);
        return;
    }
    if (state.activeAccountId && state.openedAccountIds.has(state.activeAccountId)) {
        setWebViewVisible(true);
    }
}

/**
 * Modal visibility handler: hide the native view while a dialog is open and
 * restore whatever the current page should show when it closes.
 */
function handleModalVisibility(visible) {
    if (!visible) {
        setWebViewVisible(false);
        return;
    }
    restoreWebView();
}

/**
 * Toggle the sidebar Settings badge dot to reflect whether an update is
 * known and not dismissed. Called after every render() and on update:status
 * events.
 */
function applyUpdateBadge() {
    const dot = document.querySelector('#btn-settings .update-badge');
    if (dot) {
        dot.classList.toggle('visible', !!state.updateBadge);
    }
}

/**
 * Rerender UI from current state
 */
function applyPrivacyShield() {
    const btn = document.querySelector('#btn-privacy-shield');
    if (!btn) return;
    const on = !!(state.privacyBlur && state.privacyBlur.enabled);
    btn.classList.toggle('active', on);
    btn.setAttribute('aria-pressed', on ? 'true' : 'false');
    btn.title = `Privacy Blur: ${on ? 'on' : 'off'}`;
}

function render() {
    const isSettings = state.currentView === 'settings';
    const isProfiles = state.currentView === 'profiles';
    const effectiveActiveId = isSettings ? '__settings__' : isProfiles ? '__profiles__' : state.activeAccountId;

    renderSidebar(
        sidebarContainer,
        state.accounts,
        effectiveActiveId,
        state.switchingAccountId,
        {
            onSelectAccount: handleSelectAccount,
            onAddAccount: handleOpenAddModal,
            onSettings: handleOpenSettings,
            onProfileManagement: handleOpenProfileManagement,
            privacyBlurEnabled: !!(state.privacyBlur && state.privacyBlur.enabled),
            onTogglePrivacy: handleTogglePrivacyBlur,
        },
        state.avatars
    );
    applyUpdateBadge();
    applyPrivacyShield();

    if (isSettings) {
        // Hide native WebView so the settings page is visible
        setWebViewVisible(false);
        contentView.className = 'settings-shell-view';
        renderSettingsPage(contentView, {
            onCloseToTrayChange: handleCloseToTrayChange,
            onSmartRAMSaverChange: handleSmartRAMSaverChange,
            privacyBlur: state.privacyBlur,
            onPrivacyBlurChange: handlePrivacyBlurChange,
        });
        updateUsageChip();
        return;
    }

    if (isProfiles) {
        // Hide native WebView so the profile management page is visible
        setWebViewVisible(false);
        contentView.className = 'settings-shell-view';
        renderProfilesPage(contentView, state.accounts, state.activeAccountId, {
            onRename: handleRenameAccount,
            onRemove: handleRemoveAccount,
        }, state.avatars);
        if (state.resourceUsage) updateProfilesUsage(state.resourceUsage);
        updateUsageChip();
        return;
    }

    contentView.className = 'empty-state-view';

    if (state.accounts.length === 0) {
        renderEmptyState(contentView, handleOpenAddModal);
    } else if (state.switchingAccountId) {
        const target = state.accounts.find(a => a.id === state.switchingAccountId);
        contentView.innerHTML = `
            <div class="empty-state-view">
                <div class="mini-spinner" style="width: 32px; height: 32px;"></div>
                <p class="empty-state-desc" style="margin-top: 18px;">
                    Loading ${escapeHTML(target ? target.name : 'WhatsApp Web')}...
                </p>
            </div>
        `;
    } else {
        // The native WebView2 child window covers this region entirely once the
        // session is live; this only shows through if the session failed.
        contentView.innerHTML = `
            <div class="empty-state-view">
                <div class="empty-state-icon">📱</div>
                <p class="empty-state-desc">
                    The WhatsApp Web view for the selected account is not showing.
                    Try switching to another account and back.
                </p>
            </div>
        `;
    }
    updateUsageChip();
}

/**
 * Switch account flow
 */
async function handleSelectAccount(accountId, options = {}) {
    const isRestore = options.restore === true;
    if (state.switching) {
        if (!isRestore) showToast('Account switching in progress, please wait...', 'info');
        return;
    }

    // Leaving a content-owning view (settings, profile management) returns
    // to the account view.
    const wasInContentView = isContentOwningView(state.currentView);
    state.currentView = 'account';

    if (state.activeAccountId === accountId && !isRestore && !wasInContentView) {
        return;
    }

    const api = getApp();
    if (!api) return;

    // Coming back from a content-owning view to the account that is already
    // active: nothing to switch, just bring its session back on screen.
    if (wasInContentView && state.activeAccountId === accountId && state.openedAccountIds.has(accountId)) {
        restoreWebView();
        render();
        return;
    }

    const isAlreadyOpened = state.openedAccountIds.has(accountId);

    state.switching = true;
    if (isAlreadyOpened) {
        // Instant feedback on the sidebar; no spinner, the session is live.
        state.activeAccountId = accountId;
    } else {
        state.switchingAccountId = accountId;
    }
    // Stay hidden while the switch is in flight; the correct session is
    // re-shown only after the backend finishes, so the old account never
    // flashes over the placeholder.
    render();

    try {
        await api.SwitchAccount(accountId);
        state.activeAccountId = accountId;
        state.openedAccountIds.add(accountId);
        if (!isRestore && !isAlreadyOpened) {
            showToast('Switched account successfully', 'success');
        }
    } catch (err) {
        console.error('Failed to switch account:', err);
        showToast('Switch failed: ' + err, 'error');
    } finally {
        state.switching = false;
        state.switchingAccountId = null;
        // Re-show the right view even when the switch failed: otherwise the
        // native session stays hidden behind an empty account page.
        restoreWebView();
        render();
    }
}

/**
 * Add Account modal flow
 */
function handleOpenAddModal() {
    showAddAccountModal({
        onVisibilityChange: handleModalVisibility,
        onConfirm: async (name) => {
            const api = getApp();
            if (!api) return;
            try {
                const newAcc = await api.CreateAccount(name);
                showToast(`Created account "${newAcc.name || name}"`, 'success');
                await refreshAccounts();
                // Automatically switch to the newly created account
                if (newAcc && newAcc.id) {
                    await handleSelectAccount(newAcc.id);
                }
            } catch (err) {
                console.error('Create account failed:', err);
                showToast('Failed to create account: ' + err, 'error');
            }
        },
    });
}

/**
 * Persist a full privacy blur settings object. Returns true on success so the
 * settings page can decide whether to keep the optimistic UI.
 */
async function handlePrivacyBlurChange(full) {
    const api = getApp();
    if (!api || !api.SetPrivacyBlurSettings) return false;
    try {
        await api.SetPrivacyBlurSettings(full);
        state.privacyBlur = full;
        applyPrivacyShield();
        return true;
    } catch (err) {
        console.error('Failed to save privacy blur settings:', err);
        showToast('Failed to save privacy blur settings: ' + err, 'error');
        return false;
    }
}

async function handleTogglePrivacyBlur() {
    if (!state.privacyBlur) {
        const api = getApp();
        if (api && api.GetPrivacyBlurSettings) {
            try {
                state.privacyBlur = await api.GetPrivacyBlurSettings();
            } catch (err) {
                console.error('Failed to load privacy blur settings:', err);
                return;
            }
        }
    }
    if (!state.privacyBlur) return;
    const ok = await handlePrivacyBlurChange({ ...state.privacyBlur, enabled: !state.privacyBlur.enabled });
    if (ok) {
        showToast(state.privacyBlur.enabled ? 'Privacy Blur on' : 'Privacy Blur off', 'success');
    }
}

function handleOpenSettings() {
    state.currentView = 'settings';
    render();
}

function handleOpenProfileManagement() {
    state.currentView = 'profiles';
    render();
}

function handleRenameAccount(account) {
    showRenameModal({
        currentName: account.name,
        onVisibilityChange: handleModalVisibility,
        onConfirm: async (name) => {
            const api = getApp();
            if (!api) return;
            try {
                await api.RenameAccount(account.id, name);
                showToast(`Renamed account to "${name}"`, 'success');
                await refreshAccounts();
            } catch (err) {
                console.error('Rename account failed:', err);
                showToast('Failed to rename account: ' + err, 'error');
            }
        },
    });
}

function handleRemoveAccount(account) {
    showConfirmModal({
        title: 'Remove Account',
        message: `Remove "${account.name}" and delete its isolated browser profile? This cannot be undone.`,
        confirmText: 'Remove',
        isDanger: true,
        onVisibilityChange: handleModalVisibility,
        onConfirm: async () => {
            const api = getApp();
            if (!api) return;
            try {
                await api.RemoveAccount(account.id);
                state.openedAccountIds.delete(account.id);
                showToast(`Removed account "${account.name}"`, 'success');
                await refreshAccounts();
            } catch (err) {
                console.error('Remove account failed:', err);
                showToast('Failed to remove account: ' + err, 'error');
            }
        },
    });
}

async function handleCloseToTrayChange(enabled) {
    const api = getApp();
    if (api && api.SetCloseToTray) {
        try {
            await api.SetCloseToTray(enabled);
            showToast(
                enabled ? 'Close to tray enabled' : 'Close to tray disabled',
                'success'
            );
            return true;
        } catch (err) {
            console.error('Failed to save settings:', err);
            showToast('Failed to save settings: ' + err, 'error');
            return false;
        }
    }
    return false;
}

async function handleSmartRAMSaverChange(enabled) {
    const api = getApp();
    if (api && api.SetSmartRAMSaver) {
        try {
            await api.SetSmartRAMSaver(enabled);
            showToast(
                enabled ? 'Smart RAM Saver enabled' : 'Smart RAM Saver disabled',
                'success'
            );
            return true;
        } catch (err) {
            console.error('Failed to save settings:', err);
            showToast('Failed to save settings: ' + err, 'error');
            return false;
        }
    }
    return false;
}

/**
 * Compact byte formatter for the sidebar footprint chip (MB-first).
 */
function formatUsageBytes(bytes) {
    if (!bytes || bytes <= 0) return '0 MB';
    const mb = bytes / (1024 * 1024);
    if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
    if (mb >= 1) return `${mb >= 100 ? Math.round(mb) : mb.toFixed(1)} MB`;
    return `${Math.round(bytes / 1024)} KB`;
}

/**
 * Paints the latest resource sample into the sidebar chip. The chip is
 * re-created by every sidebar re-render, so this is safe to call any time.
 */
function updateUsageChip() {
    const chip = document.querySelector('#sidebar-usage-chip');
    if (!chip) return;
    const usage = state.resourceUsage;
    const ram = chip.querySelector('.usage-ram');
    const cpu = chip.querySelector('.usage-cpu');
    if (!usage || (!usage.totalWorkingSetBytes && !usage.totalCPUPercent)) {
        if (ram) ram.textContent = '– MB';
        if (cpu) cpu.textContent = '–%';
        return;
    }
    if (ram) ram.textContent = formatUsageBytes(usage.totalWorkingSetBytes || 0);
    if (cpu) cpu.textContent = `${(usage.totalCPUPercent || 0).toFixed(1)}% CPU`;
    chip.title = `Wassap Desktop — total ${formatUsageBytes(usage.totalWorkingSetBytes || 0)} RAM, ${(usage.totalCPUPercent || 0).toFixed(1)}% CPU` +
        ` (app ${formatUsageBytes(usage.appWorkingSetBytes || 0)}, profiles ${formatUsageBytes((usage.totalWorkingSetBytes || 0) - (usage.appWorkingSetBytes || 0))})`;
}

/**
 * Polls the backend for process stats. Drives the sidebar chip and, while
 * the Profile Management page is open, its per-account breakdown too.
 */
function startResourceUsagePolling() {
    const tick = async () => {
        const api = getApp();
        if (!api || !api.GetResourceUsage) return;
        try {
            const usage = await api.GetResourceUsage();
            state.resourceUsage = normalizeUsage(usage);
        } catch (err) {
            console.error('Failed to sample resource usage:', err);
            return;
        }
        updateUsageChip();
        if (state.currentView === 'profiles') {
            updateProfilesUsage(state.resourceUsage);
        }
    };
    tick();
    setInterval(tick, 2000);
}

/**
 * Wails returns Go structs with the JSON keys we tagged; normalize the few
 * numeric fields we rely on so the UI never formats undefined.
 */
function normalizeUsage(raw) {
    const u = raw || {};
    const profiles = Array.isArray(u.profiles) ? u.profiles : [];
    return {
        appWorkingSetBytes: Number(u.appWorkingSetBytes) || 0,
        appCPUPercent: Number(u.appCPUPercent) || 0,
        totalWorkingSetBytes: Number(u.totalWorkingSetBytes) || 0,
        totalCPUPercent: Number(u.totalCPUPercent) || 0,
        profiles: profiles.map(p => ({
            id: p.id,
            running: !!p.running,
            processCount: Number(p.processCount) || 0,
            workingSetBytes: Number(p.workingSetBytes) || 0,
            cpuPercent: Number(p.cpuPercent) || 0,
        })),
    };
}

/**
 * Refresh account list from backend with retry until bridge is ready
 */
async function refreshAccounts(retryCount = 0) {
    const api = getApp();
    if (!api) {
        if (retryCount < 20) {
            setTimeout(() => refreshAccounts(retryCount + 1), 250);
        }
        return;
    }

    try {
        const rawAccounts = await api.ListAccounts();
        state.accounts = (rawAccounts || []).map(a => ({
            id: a.ID || a.id,
            name: a.Name || a.name,
            createdAt: a.CreatedAt || a.createdAt,
        }));
        state.avatars = await fetchAvatars(api, state.accounts.map(a => a.id));

        const activeId = await api.GetActiveAccountID();
        state.activeAccountId = activeId || null;
        state.isLoading = false;
        render();
    } catch (err) {
        console.error('Failed to load accounts:', err);
        showToast('Failed to load accounts: ' + err, 'error');
        state.isLoading = false;
        render();
    }
}

/**
 * Observe window resize and update backend
 */
let resizePending = false;
function setupResizeObserver() {
    window.addEventListener('resize', () => {
        if (resizePending) return;
        resizePending = true;
        requestAnimationFrame(() => {
            resizePending = false;
            const api = getApp();
            if (api && api.NotifyWindowResize) {
                api.NotifyWindowResize(window.innerWidth, window.innerHeight).catch(() => {});
            }
        });
    });
}

function escapeHTML(str) {
    if (!str) return '';
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

// Initial bootstrap
window.addEventListener('DOMContentLoaded', () => {
    setupResizeObserver();
    startResourceUsagePolling();

    // Desktop shell has no browser chrome: suppress the WebView2 default
    // context menu everywhere, except in text fields where paste/inspect
    // of the input itself is still useful. Native WhatsApp Web content is
    // a separate window and unaffected by this.
    document.addEventListener('contextmenu', (e) => {
        if (e.target.closest('input, textarea, [contenteditable="true"]')) {
            return;
        }
        e.preventDefault();
    });

    // Listen for tray menu "Settings" action
    if (window.runtime && window.runtime.EventsOn) {
        window.runtime.EventsOn('open-settings', () => {
            handleOpenSettings();
        });
        // The backend emits this when a desktop notification is clicked:
        // bring the app forward and switch to the account that sent it.
        window.runtime.EventsOn('notification:activate', (accountId) => {
            if (accountId) {
                handleSelectAccount(accountId, { restore: true });
            }
        });
        // Backend → frontend update status pushes.
        window.runtime.EventsOn('update:status', (s) => {
            state.updateBadge = !!s && (s.state === 'available' || s.state === 'downloaded') && !s.dismissed;
            applyUpdateBadge();
            updateSettingsStatusRow(s);
        });
        // Privacy blur state pushes (in-page Alt+/ toggle, settings save).
        window.runtime.EventsOn('privacy-blur:changed', (s) => {
            if (!s) return;
            state.privacyBlur = s;
            applyPrivacyShield();
            updatePrivacyBlurControls(s);
        });
        // Profile photos scraped from each account's WhatsApp Web session.
        window.runtime.EventsOn('account-avatar:changed', async (change) => {
            if (!change || !change.id) return;
            if (!change.hash) {
                delete state.avatars[change.id];
                render();
                return;
            }
            const next = await fetchAvatars(getApp(), [change.id]);
            if (next[change.id]) {
                state.avatars[change.id] = next[change.id];
                render();
            }
        });
    }

    const goApp = window.go && window.go.app && window.go.app.App;
    if (goApp && goApp.GetPrivacyBlurSettings) {
        goApp.GetPrivacyBlurSettings().then((s) => {
            state.privacyBlur = s;
            applyPrivacyShield();
        }).catch(() => {});
    }
    if (goApp && goApp.GetUpdateStatus) {
        goApp.GetUpdateStatus().then((s) => {
            state.updateBadge = !!s && (s.state === 'available' || s.state === 'downloaded') && !s.dismissed;
            applyUpdateBadge();
        }).catch(() => {});
    }

    setTimeout(() => {
        (async () => {
            await refreshAccounts();
            // Auto-restore last active account's WebView on startup
            if (state.activeAccountId && !state.switchingAccountId) {
                await handleSelectAccount(state.activeAccountId, { restore: true });
            }
        })();
    }, 200);
});
