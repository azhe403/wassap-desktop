import './style.css';
import './app.css';

import { state } from './state.js';
import { renderSidebar, renderEmptyState } from './components/sidebar.js';
import { showAddAccountModal, showRenameModal, showConfirmModal } from './components/modal.js';
import { showContextMenu } from './components/context-menu.js';
import { renderSettingsPage } from './components/settings-page.js';

// Access Wails backend bindings
const getApp = () => window.go && window.go.main && window.go.main.App;

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
 * Restore the native view for the active account, unless the settings page or
 * a modal currently owns the content area.
 */
function restoreWebView() {
    if (state.currentView === 'settings') {
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
 * Rerender UI from current state
 */
function render() {
    const isSettings = state.currentView === 'settings';
    const effectiveActiveId = isSettings ? '__settings__' : state.activeAccountId;

    renderSidebar(
        sidebarContainer,
        state.accounts,
        effectiveActiveId,
        state.switchingAccountId,
        {
            onSelectAccount: handleSelectAccount,
            onContextMenu: handleContextMenu,
            onAddAccount: handleOpenAddModal,
            onSettings: handleOpenSettings,
        }
    );

    if (isSettings) {
        // Hide native WebView so the settings page is visible
        setWebViewVisible(false);
        contentView.className = 'settings-shell-view';
        renderSettingsPage(contentView, {
            onCloseToTrayChange: handleCloseToTrayChange,
        });
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

    // Leaving the settings page returns to the account view.
    const wasInSettings = state.currentView === 'settings';
    state.currentView = 'account';

    if (state.activeAccountId === accountId && !isRestore && !wasInSettings) {
        return;
    }

    const api = getApp();
    if (!api) return;

    // Coming back from the settings page to the account that is already
    // active: nothing to switch, just bring its session back on screen.
    if (wasInSettings && state.activeAccountId === accountId && state.openedAccountIds.has(accountId)) {
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
 * Rename Account modal flow
 */
function handleOpenRenameModal(account) {
    showRenameModal({
        currentName: account.name,
        onVisibilityChange: handleModalVisibility,
        onConfirm: async (newName) => {
            const api = getApp();
            if (!api) return;
            try {
                await api.RenameAccount(account.id, newName);
                showToast('Account renamed', 'success');
                await refreshAccounts();
            } catch (err) {
                console.error('Rename failed:', err);
                showToast('Rename failed: ' + err, 'error');
            }
        },
    });
}

/**
 * Remove Account flow
 */
function handleRemoveAccount(account) {
    if (account.id === state.activeAccountId) {
        showToast('Cannot remove active account. Switch to another account first.', 'error');
        return;
    }

    showConfirmModal({
        title: 'Remove Account',
        message: `Are you sure you want to remove "${account.name}"? Its browser session and storage profile will be permanently deleted.`,
        confirmText: 'Delete Permanently',
        isDanger: true,
        onVisibilityChange: handleModalVisibility,
        onConfirm: async () => {
            const api = getApp();
            if (!api) return;
            try {
                await api.RemoveAccount(account.id);
                showToast(`Account "${account.name}" removed`, 'success');
                await refreshAccounts();
            } catch (err) {
                console.error('Remove failed:', err);
                showToast('Remove failed: ' + err, 'error');
            }
        },
    });
}

/**
 * Context menu handler
 */
function handleContextMenu(x, y, account) {
    showContextMenu(x, y, account, {
        onRename: handleOpenRenameModal,
        onRemove: handleRemoveAccount,
    });
}

function handleOpenSettings() {
    state.currentView = 'settings';
    render();
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

    // Listen for tray menu "Settings" action
    if (window.runtime && window.runtime.EventsOn) {
        window.runtime.EventsOn('open-settings', () => {
            handleOpenSettings();
        });
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
