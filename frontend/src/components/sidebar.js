import { getInitials, getAvatarColor } from '../utils/avatar.js';

/**
 * Renders the vertical sidebar with account avatars.
 *
 * @param {HTMLElement} container
 * @param {Array} accounts
 * @param {string|null} activeId
 * @param {string|null} switchingId
 * @param {Object} handlers
 */
export function renderSidebar(container, accounts, activeId, switchingId, handlers) {
    container.innerHTML = `
        <aside class="app-sidebar" aria-label="Account Switcher">
            <div class="sidebar-header">
                <div class="sidebar-brand-icon" title="Wassap Desktop">
                    <span>💬</span>
                </div>
            </div>

            <div class="sidebar-divider"></div>

            <div class="account-items-scroll" role="tablist" aria-orientation="vertical">
                ${accounts.map(acc => {
                    const isActive = acc.id === activeId;
                    const isSwitching = acc.id === switchingId;
                    const color = getAvatarColor(acc.id || acc.name);
                    const initials = getInitials(acc.name);

                    return `
                        <div class="account-item-wrapper ${isActive ? 'active' : ''}" data-id="${acc.id}">
                            <div class="active-pill" aria-hidden="true"></div>
                            <button 
                                class="account-avatar-btn" 
                                style="background-color: ${color.bg}; color: ${color.fg};"
                                title="${escapeAttr(acc.name)}"
                                aria-label="${escapeAttr(acc.name)}"
                                role="tab"
                                aria-selected="${isActive ? 'true' : 'false'}"
                                data-id="${acc.id}"
                            >
                                <span>${initials}</span>
                                <span class="unread-badge" aria-hidden="true"></span>
                                ${isSwitching ? `
                                    <div class="switching-overlay" aria-label="Switching profile">
                                        <div class="mini-spinner"></div>
                                    </div>
                                ` : ''}
                            </button>
                        </div>
                    `;
                }).join('')}
            </div>

            <div class="sidebar-divider"></div>

            <div class="sidebar-footer">
                <button id="btn-add-account" class="sidebar-icon-btn" title="Add Account" aria-label="Add Account">
                    <span>➕</span>
                </button>
                <button id="btn-settings" class="sidebar-icon-btn" title="Settings" aria-label="Settings">
                    <span>⚙️</span>
                </button>
            </div>
        </aside>
    `;

    // Bind avatar clicks and right-clicks
    container.querySelectorAll('.account-avatar-btn').forEach(btn => {
        const id = btn.getAttribute('data-id');
        btn.addEventListener('click', (e) => {
            e.preventDefault();
            if (handlers.onSelectAccount) handlers.onSelectAccount(id);
        });

        btn.addEventListener('contextmenu', (e) => {
            e.preventDefault();
            const account = accounts.find(a => a.id === id);
            if (account && handlers.onContextMenu) {
                handlers.onContextMenu(e.clientX, e.clientY, account);
            }
        });
    });

    const addBtn = container.querySelector('#btn-add-account');
    if (addBtn && handlers.onAddAccount) {
        addBtn.addEventListener('click', handlers.onAddAccount);
    }

    const settingsBtn = container.querySelector('#btn-settings');
    if (settingsBtn && handlers.onSettings) {
        settingsBtn.addEventListener('click', handlers.onSettings);
    }
}

/**
 * Renders the empty state in the content area when no accounts exist.
 */
export function renderEmptyState(container, onAddAccount) {
    container.innerHTML = `
        <div class="empty-state-view">
            <div class="empty-state-icon">📱</div>
            <h2 class="empty-state-title">Welcome to Wassap Desktop</h2>
            <p class="empty-state-desc">
                No WhatsApp accounts connected yet. Add your first account to start chatting with isolated profile storage.
            </p>
            <button id="btn-empty-add" class="btn-primary-action">
                <span>➕</span>
                <span>Add WhatsApp Account</span>
            </button>
        </div>
    `;

    const btn = container.querySelector('#btn-empty-add');
    if (btn && onAddAccount) {
        btn.addEventListener('click', onAddAccount);
    }
}

function escapeAttr(str) {
    if (!str) return '';
    return str.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}
