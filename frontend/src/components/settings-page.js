/**
 * Renders the full-page Settings view inside #content-view.
 *
 * @param {HTMLElement} container
 * @param {Object} handlers - { onCloseToTrayChange }
 */
export function renderSettingsPage(container, handlers = {}) {
    container.innerHTML = `
        <div class="settings-page-view" role="region" aria-label="Settings">
            <div class="settings-page-header">
                <h2 class="settings-page-title">Settings</h2>
                <p class="settings-page-subtitle">Configure Wassap Desktop preferences</p>
            </div>

            <div class="settings-section-title">General</div>
            <div class="settings-group">
                <label class="settings-toggle-row" for="setting-close-to-tray-page">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Close to Tray</span>
                        <span class="settings-toggle-desc">
                            Minimize application to the system tray when closing the window instead of quitting.
                        </span>
                    </div>
                    <input 
                        id="setting-close-to-tray-page" 
                        class="settings-checkbox" 
                        type="checkbox" 
                    />
                </label>
            </div>

            <div class="settings-section-title">Software Update</div>
            <div class="settings-group">
                <label class="settings-toggle-row" for="setting-auto-update">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Auto-update</span>
                        <span class="settings-toggle-desc">
                            Checks for new releases and downloads them automatically. Installs only on restart.
                        </span>
                    </div>
                    <input
                        id="setting-auto-update"
                        class="settings-checkbox"
                        type="checkbox"
                    />
                </label>

                <div class="settings-toggle-row" style="cursor: default;">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Update channel</span>
                        <span class="settings-toggle-desc">
                            Stable follows tagged releases. Nightly follows main and may break.
                        </span>
                    </div>
                    <select id="setting-update-channel" class="settings-select">
                        <option value="stable">Stable (recommended)</option>
                        <option value="nightly">Nightly (bleeding edge)</option>
                    </select>
                </div>

                <div class="update-status-row">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Wassap Desktop <span id="update-current-version"></span></span>
                        <span class="settings-toggle-desc" id="update-last-checked">Not checked yet</span>
                        <span class="update-status-text" id="update-status-text"></span>
                        <div class="update-status-actions">
                            <button id="btn-update-check" class="update-action-btn update-action-secondary" type="button">Check for updates</button>
                            <button id="btn-update-action" class="update-action-btn" type="button" style="display:none;"></button>
                            <button id="btn-update-dismiss" class="update-link-btn" type="button" style="display:none;">Dismiss</button>
                        </div>
                    </div>
                </div>
            </div>

            <div class="settings-section-title" style="margin-top: 32px;">About</div>
            <div class="settings-group">
                <div class="settings-toggle-row" style="cursor: default;">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Wassap Desktop</span>
                        <span class="settings-toggle-desc">
                            Multi-account WhatsApp client with isolated profile storage.
                        </span>
                    </div>
                    <span id="about-version" style="font-size: 12px; color: var(--text-secondary);">dev</span>
                </div>
            </div>
        </div>
    `;

    const checkbox = container.querySelector('#setting-close-to-tray-page');

    // Load persisted value asynchronously
    const api = window.go && window.go.main && window.go.main.App;
    if (api && api.GetCloseToTray) {
        api.GetCloseToTray().then((val) => {
            if (checkbox) checkbox.checked = !!val;
        }).catch((err) => {
            console.error('Failed to load close-to-tray setting:', err);
        });
    }

    if (checkbox) {
        checkbox.addEventListener('change', async (e) => {
            const enabled = e.target.checked;
            if (handlers.onCloseToTrayChange) {
                const ok = await handlers.onCloseToTrayChange(enabled);
                if (ok === false) {
                    // Revert so the UI does not claim a setting that never
                    // reached disk.
                    e.target.checked = !enabled;
                }
            }
        });
    }

    // --- Software Update section ----------------------------------------

    const autoUpdate = container.querySelector('#setting-auto-update');
    const channelSelect = container.querySelector('#setting-update-channel');
    const checkBtn = container.querySelector('#btn-update-check');
    const actionBtn = container.querySelector('#btn-update-action');
    const dismissBtn = container.querySelector('#btn-update-dismiss');

    if (api && api.GetAutoUpdate && autoUpdate) {
        api.GetAutoUpdate().then((val) => {
            autoUpdate.checked = !!val;
        }).catch((err) => {
            console.error('Failed to load auto-update setting:', err);
        });
    }
    if (autoUpdate) {
        autoUpdate.addEventListener('change', async (e) => {
            const enabled = e.target.checked;
            try {
                if (api.SetAutoUpdate) await api.SetAutoUpdate(enabled);
            } catch (err) {
                console.error('Failed to save auto-update:', err);
                e.target.checked = !enabled;
            }
        });
    }

    if (api && api.GetUpdateChannel && channelSelect) {
        api.GetUpdateChannel().then((val) => {
            if (val === 'stable' || val === 'nightly') channelSelect.value = val;
        }).catch((err) => {
            console.error('Failed to load update channel:', err);
        });
    }
    if (channelSelect) {
        channelSelect.addEventListener('change', async (e) => {
            const previous = e.target.value === 'nightly' ? 'stable' : 'nightly';
            const value = e.target.value;
            try {
                if (api.SetUpdateChannel) await api.SetUpdateChannel(value);
            } catch (err) {
                console.error('Failed to save update channel:', err);
                e.target.value = previous;
            }
        });
    }

    const aboutVersion = container.querySelector('#about-version');
    if (api && api.GetVersion && aboutVersion) {
        api.GetVersion().then((v) => {
            aboutVersion.textContent = `v${v.version}${v.commit ? ' (' + v.commit.slice(0, 7) + ')' : ''}`;
        }).catch((err) => {
            console.error('Failed to load version:', err);
        });
    }

    if (api && api.GetUpdateStatus) {
        api.GetUpdateStatus().then((s) => renderUpdateStatus(container, s)).catch((err) => {
            console.error('Failed to load update status:', err);
        });
    }

    if (checkBtn) {
        checkBtn.addEventListener('click', async () => {
            const original = checkBtn.textContent;
            checkBtn.disabled = true;
            checkBtn.textContent = 'Checking…';
            try {
                if (api.CheckForUpdates) {
                    const s = await api.CheckForUpdates();
                    renderUpdateStatus(container, s);
                }
            } catch (err) {
                console.error('Update check failed:', err);
                const textEl = container.querySelector('#update-status-text');
                if (textEl) {
                    textEl.textContent = `Check failed: ${err}`;
                    textEl.classList.add('error');
                }
            } finally {
                checkBtn.disabled = false;
                checkBtn.textContent = original;
            }
        });
    }

    if (actionBtn) {
        actionBtn.addEventListener('click', async () => {
            const action = actionBtn.dataset.action;
            actionBtn.disabled = true;
            try {
                if (action === 'download' && api.DownloadUpdate) {
                    await api.DownloadUpdate();
                    const s = await api.GetUpdateStatus();
                    renderUpdateStatus(container, s);
                } else if (action === 'restart' && api.RestartToApplyUpdate) {
                    await api.RestartToApplyUpdate();
                    // On success the app swaps the binary and quits; if the
                    // call returns, something went wrong before the quit.
                    actionBtn.disabled = false;
                }
            } catch (err) {
                console.error('Update action failed:', err);
                const textEl = container.querySelector('#update-status-text');
                if (textEl) {
                    textEl.textContent = `Update failed: ${err}`;
                    textEl.classList.add('error');
                }
                actionBtn.disabled = false;
            }
        });
    }

    if (dismissBtn) {
        dismissBtn.addEventListener('click', async () => {
            const s = container.__updateStatus;
            if (!s || !api.DismissUpdateNotice) return;
            try {
                await api.DismissUpdateNotice(s.latestVersion);
                renderUpdateStatus(container, { ...s, dismissed: true });
            } catch (err) {
                console.error('Failed to dismiss update notice:', err);
            }
        });
    }
}

/**
 * Renders the update status row from an UpdateStatus payload (also called
 * on `update:status` Wails events from main.js).
 */
export function renderUpdateStatus(container, s) {
    if (!s) return;
    container.__updateStatus = s;

    const current = container.querySelector('#update-current-version');
    const lastChecked = container.querySelector('#update-last-checked');
    const textEl = container.querySelector('#update-status-text');
    const actionBtn = container.querySelector('#btn-update-action');
    const dismissBtn = container.querySelector('#btn-update-dismiss');
    if (!textEl) return;

    if (current) {
        current.textContent = s.currentVersion ? `v${s.currentVersion}` : '';
    }
    if (lastChecked) {
        lastChecked.textContent = s.checkedAt
            ? `Last checked: ${new Date(s.checkedAt).toLocaleString()}`
            : 'Not checked yet';
    }

    const latest = s.latestVersion || '';
    const pct = s.progressTotal > 0 ? Math.floor((s.progressDone * 100) / s.progressTotal) : null;
    let text = '';
    let tone = '';
    let action = null;
    let actionLabel = '';
    let showDismiss = false;

    switch (s.state) {
        case 'up-to-date':
            text = "You're on the latest version.";
            break;
        case 'available':
            text = s.dismissed
                ? `v${latest} is available. (notice dismissed)`
                : `v${latest} is available.`;
            action = 'download';
            actionLabel = `Update to v${latest}`;
            showDismiss = !s.dismissed;
            break;
        case 'downloading':
            text = pct !== null ? `Downloading… ${pct}%` : 'Downloading…';
            break;
        case 'downloaded':
            text = `v${latest} downloaded. Restart to apply.`;
            action = 'restart';
            actionLabel = 'Restart to update';
            break;
        case 'error':
            text = s.message || 'Update check failed.';
            tone = 'error';
            break;
        case 'unsupported':
            text = "Update checks aren't supported on this build.";
            break;
        case 'unknown':
            text = 'Not checked yet.';
            break;
        default:
            text = s.message || '';
    }

    textEl.textContent = text;
    textEl.classList.toggle('error', tone === 'error');

    if (actionBtn) {
        if (action) {
            actionBtn.style.display = '';
            actionBtn.dataset.action = action;
            actionBtn.textContent = actionLabel;
            actionBtn.disabled = false;
        } else {
            actionBtn.style.display = 'none';
            actionBtn.removeAttribute('data-action');
        }
    }
    if (dismissBtn) {
        dismissBtn.style.display = showDismiss ? '' : 'none';
    }
}

/**
 * Applies an `update:status` event payload to the status row when the
 * settings page is currently in the DOM. Called from main.js.
 */
export function updateSettingsStatusRow(s) {
    if (typeof document === 'undefined') return;
    const container = document.querySelector('.settings-page-view');
    if (!container) return;
    renderUpdateStatus(container, s);
}

