import { getInitials, getAvatarColor } from '../utils/avatar.js';
import { avatarImageHTML } from '../utils/avatar-image.js';
import logoIcon from '../assets/images/logo-universal.png';

/**
 * Renders the vertical sidebar with account avatars.
 *
 * @param {HTMLElement} container
 * @param {Array} accounts
 * @param {string|null} activeId
 * @param {string|null} switchingId
 * @param {Object} handlers
 * @param {Object} avatars - account id -> profile photo data URI
 */
export function renderSidebar(container, accounts, activeId, switchingId, handlers, avatars = {}) {
    container.innerHTML = `
        <aside class="app-sidebar" aria-label="Account Switcher">
            <div class="sidebar-header">
                <button id="btn-profile-management" class="sidebar-brand-icon sidebar-brand-btn ${activeId === '__profiles__' ? 'active' : ''}" title="Profile Management" aria-label="Profile Management" style="background: none;">
                    <img src="${logoIcon}" alt="Wassap" style="width: 36px; height: 36px; border-radius: 8px; object-fit: contain;" />
                </button>
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
                                <span>${escapeHTML(initials)}</span>
                                ${avatarImageHTML(acc.id, avatars, '', 'account-avatar-img')}
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
                <button id="btn-privacy-shield" class="sidebar-shield-btn ${handlers.privacyBlurEnabled ? 'active' : ''}" title="Privacy Blur: ${handlers.privacyBlurEnabled ? 'on' : 'off'}" aria-pressed="${handlers.privacyBlurEnabled ? 'true' : 'false'}" aria-label="Toggle Privacy Blur">
                    <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                        <path d="M12 3l7 2.8v5.4c0 4.6-3 7.9-7 9.8-4-1.9-7-5.2-7-9.8V5.8L12 3z"/>
                        <path d="M9.5 12l2 2 3.5-3.5"/>
                    </svg>
                </button>
                <button id="btn-settings" class="sidebar-icon-btn ${activeId === '__settings__' ? 'active' : ''}" title="Settings" aria-label="Settings">
                    <span>⚙️</span>
                    <span class="update-badge" aria-hidden="true"></span>
                </button>
                <div id="sidebar-usage-chip" class="sidebar-usage-chip" role="status" title="Wassap Desktop total resource usage">
                    <span class="usage-ram">– MB</span>
                    <span class="usage-cpu">–%</span>
                </div>
            </div>
        </aside>
    `;

    // Bind avatar clicks
    container.querySelectorAll('.account-avatar-btn').forEach(btn => {
        const id = btn.getAttribute('data-id');
        btn.addEventListener('click', (e) => {
            e.preventDefault();
            if (handlers.onSelectAccount) handlers.onSelectAccount(id);
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

    const shieldBtn = container.querySelector('#btn-privacy-shield');
    if (shieldBtn && handlers.onTogglePrivacy) {
        shieldBtn.addEventListener('click', handlers.onTogglePrivacy);
    }

    const profilesBtn = container.querySelector('#btn-profile-management');
    if (profilesBtn && handlers.onProfileManagement) {
        profilesBtn.addEventListener('click', handlers.onProfileManagement);
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

function escapeHTML(str) {
    if (!str) return '';
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function escapeAttr(str) {
    if (!str) return '';
    return str.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}
