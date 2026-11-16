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

            <div class="settings-section-title">Privacy Blur</div>
            <div class="settings-group" id="settings-privacy-blur-group">
                <label class="settings-toggle-row privacy-master-row" for="setting-privacy-blur-enabled">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Privacy Blur</span>
                        <span class="settings-toggle-desc">
                            Blur contacts, chats and media until you hover them. Applies to every account. Alt+/ toggles it in a chat.
                        </span>
                    </div>
                    <input
                        id="setting-privacy-blur-enabled"
                        class="settings-checkbox"
                        type="checkbox"
                    />
                </label>

                <div class="settings-toggle-row" style="cursor: default;">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Mode</span>
                        <span class="settings-toggle-desc">
                            Blur reveals on hover; Redacted blocks everything out black.
                        </span>
                    </div>
                    <div class="privacy-mode-segmented" role="group" aria-label="Privacy mode">
                        <button type="button" class="privacy-mode-btn" data-mode="blur">Blur</button>
                        <button type="button" class="privacy-mode-btn" data-mode="redacted">Redacted</button>
                    </div>
                </div>

                <label class="settings-toggle-row" for="setting-privacy-blur-amount">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Blur strength <span id="privacy-blur-amount-value" class="settings-value-chip">3px</span></span>
                        <span class="settings-toggle-desc">Higher values hide more detail.</span>
                    </div>
                    <input id="setting-privacy-blur-amount" class="settings-range" type="range" min="1" max="10" step="1" />
                </label>

                <label class="settings-toggle-row" for="setting-privacy-avatars">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Profile photos</span>
                        <span class="settings-toggle-desc">Avatars and status rings in the chat list and headers.</span>
                    </div>
                    <input id="setting-privacy-avatars" class="settings-checkbox" type="checkbox" data-key="blurAvatars" />
                </label>

                <label class="settings-toggle-row" for="setting-privacy-names">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Contact &amp; group names</span>
                        <span class="settings-toggle-desc">Names in the chat list, chat header and group info.</span>
                    </div>
                    <input id="setting-privacy-names" class="settings-checkbox" type="checkbox" data-key="blurNames" />
                </label>

                <label class="settings-toggle-row" for="setting-privacy-previews">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Chat previews</span>
                        <span class="settings-toggle-desc">Last-message previews in the chat list.</span>
                    </div>
                    <input id="setting-privacy-previews" class="settings-checkbox" type="checkbox" data-key="blurPreviews" />
                </label>

                <label class="settings-toggle-row" for="setting-privacy-bubbles">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Open chat messages</span>
                        <span class="settings-toggle-desc">Message bubbles in the open conversation.</span>
                    </div>
                    <input id="setting-privacy-bubbles" class="settings-checkbox" type="checkbox" data-key="blurTextChats" />
                </label>

                <label class="settings-toggle-row" for="setting-privacy-stickers">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Stickers</span>
                        <span class="settings-toggle-desc">Stickers in chats.</span>
                    </div>
                    <input id="setting-privacy-stickers" class="settings-checkbox" type="checkbox" data-key="blurStickers" />
                </label>

                <label class="settings-toggle-row" for="setting-privacy-media">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Media previews</span>
                        <span class="settings-toggle-desc">Images, videos and document previews in chats.</span>
                    </div>
                    <input id="setting-privacy-media" class="settings-checkbox" type="checkbox" data-key="blurMediaPreview" />
                </label>

                <label class="settings-toggle-row" for="setting-privacy-gallery">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Media gallery</span>
                        <span class="settings-toggle-desc">The fullscreen media viewer.</span>
                    </div>
                    <input id="setting-privacy-gallery" class="settings-checkbox" type="checkbox" data-key="blurMediaGallery" />
                </label>

                <label class="settings-toggle-row" for="setting-privacy-input">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Typing field</span>
                        <span class="settings-toggle-desc">Fade the composer instead of blurring it. Uses the opacity slider below.</span>
                    </div>
                    <input id="setting-privacy-input" class="settings-checkbox" type="checkbox" data-key="blurInput" />
                </label>

                <label class="settings-toggle-row" for="setting-privacy-input-opacity">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Typing field opacity <span id="privacy-input-opacity-value" class="settings-value-chip">30%</span></span>
                        <span class="settings-toggle-desc">How visible the text you type is while Privacy Blur is on.</span>
                    </div>
                    <input id="setting-privacy-input-opacity" class="settings-range" type="range" min="0" max="100" step="5" />
                </label>

                <label class="settings-toggle-row" for="setting-privacy-lastn">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Keep last messages visible</span>
                        <span class="settings-toggle-desc">Always show the newest messages in the open chat, blur the rest.</span>
                    </div>
                    <input id="setting-privacy-lastn" class="settings-checkbox" type="checkbox" data-key="unblurLastN" />
                </label>

                <div class="settings-toggle-row" style="cursor: default;">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Visible messages</span>
                        <span class="settings-toggle-desc">How many of the newest messages stay readable.</span>
                    </div>
                    <select id="setting-privacy-lastn-count" class="settings-select">
                        <option value="1">1</option>
                        <option value="2">2</option>
                        <option value="3">3</option>
                        <option value="4">4</option>
                        <option value="5">5</option>
                        <option value="6">6</option>
                        <option value="7">7</option>
                        <option value="8">8</option>
                        <option value="9">9</option>
                        <option value="10">10</option>
                    </select>
                </div>

                <div class="settings-privacy-footnote">Changes apply immediately to open chats. Nothing leaves this device.</div>
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

    // --- Privacy Blur section ----------------------------------------

    const privacyGroup = container.querySelector('#settings-privacy-blur-group');
    const privacyMaster = container.querySelector('#setting-privacy-blur-enabled');
    const privacyAmount = container.querySelector('#setting-privacy-blur-amount');
    const privacyAmountValue = container.querySelector('#privacy-blur-amount-value');
    const privacyInputOpacity = container.querySelector('#setting-privacy-input-opacity');
    const privacyInputOpacityValue = container.querySelector('#privacy-input-opacity-value');
    const privacyLastNCount = container.querySelector('#setting-privacy-lastn-count');
    const privacyModeButtons = container.querySelectorAll('.privacy-mode-btn');

    // The settings object the page renders and mutates locally; replaced on
    // every blur:changed event and on every successful save.
    let privacy = handlers.privacyBlur || null;

    function applyPrivacy(settings) {
        privacy = settings;
        updatePrivacyBlurControls(settings);
    }

    async function commitPrivacy(next) {
        if (handlers.onPrivacyBlurChange) {
            const ok = await handlers.onPrivacyBlurChange(next);
            if (ok === false && api && api.GetPrivacyBlurSettings) {
                // Revert the optimistic UI to what the backend actually stored.
                try {
                    applyPrivacy(await api.GetPrivacyBlurSettings());
                } catch (err) {
                    console.error('Failed to re-read privacy blur settings:', err);
                }
            } else if (ok !== false) {
                applyPrivacy(next);
            }
        }
    }

    function commitKey(key) {
        return (event) => {
            if (!privacy) return;
            const target = event.target;
            const value = target.type === 'checkbox' ? target.checked : target.value;
            commitPrivacy({ ...privacy, [key]: value });
        };
    }

    if (handlers.onPrivacyBlurChange) {
        container.querySelectorAll('#settings-privacy-blur-group [data-key]').forEach((el) => {
            el.addEventListener('change', commitKey(el.getAttribute('data-key')));
        });
    }

    if (privacyMaster) {
        privacyMaster.addEventListener('change', (e) => {
            if (!privacy) return;
            commitPrivacy({ ...privacy, enabled: e.target.checked });
        });
    }

    privacyModeButtons.forEach((btn) => {
        btn.addEventListener('click', () => {
            if (!privacy) return;
            commitPrivacy({ ...privacy, privacyMode: btn.getAttribute('data-mode') });
        });
    });

    if (privacyAmount) {
        privacyAmount.addEventListener('input', () => {
            if (privacyAmountValue) privacyAmountValue.textContent = `${privacyAmount.value}px`;
        });
        privacyAmount.addEventListener('change', () => {
            if (!privacy) return;
            commitPrivacy({ ...privacy, blurAmount: Number(privacyAmount.value) });
        });
    }

    if (privacyInputOpacity) {
        privacyInputOpacity.addEventListener('input', () => {
            if (privacyInputOpacityValue) privacyInputOpacityValue.textContent = `${privacyInputOpacity.value}%`;
        });
        privacyInputOpacity.addEventListener('change', () => {
            if (!privacy) return;
            commitPrivacy({ ...privacy, inputOpacity: Number(privacyInputOpacity.value) });
        });
    }

    if (privacyLastNCount) {
        privacyLastNCount.addEventListener('change', () => {
            if (!privacy) return;
            commitPrivacy({ ...privacy, unblurLastNCount: Number(privacyLastNCount.value) });
        });
    }

    // Seed from the handler-provided object (render() passes state.privacyBlur)
    // and refresh from the backend so a reopened page never shows stale rows.
    if (privacy) {
        updatePrivacyBlurControls(privacy);
    }
    if (api && api.GetPrivacyBlurSettings) {
        api.GetPrivacyBlurSettings().then((s) => {
            applyPrivacy(s);
        }).catch((err) => {
            console.error('Failed to load privacy blur settings:', err);
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
 * Applies a PrivacyBlurConfig object to the section's controls, dimming the
 * sub-rows when the master switch is off. Safe to call when the settings
 * page is not in the DOM. Called from renderSettingsPage and from main.js on
 * every `privacy-blur:changed` event.
 */
export function updatePrivacyBlurControls(settings) {
    if (typeof document === 'undefined') return;
    const group = document.querySelector('#settings-privacy-blur-group');
    if (!group || !settings) return;

    const enabled = !!settings.enabled;
    group.classList.toggle('settings-group-disabled', !enabled);

    // Keyboard/AT operability must match the visual dimming: the sub-controls
    // are really disabled, not just opacity-hidden.
    group.querySelectorAll('[data-key], #setting-privacy-blur-amount, #setting-privacy-input-opacity, #setting-privacy-lastn-count, .privacy-mode-btn').forEach((el) => {
        el.disabled = !enabled;
    });

    const master = group.querySelector('#setting-privacy-blur-enabled');
    if (master) master.checked = enabled;

    group.querySelectorAll('[data-key]').forEach((el) => {
        el.checked = !!settings[el.getAttribute('data-key')];
    });

    const amount = group.querySelector('#setting-privacy-blur-amount');
    if (amount) {
        amount.value = String(settings.blurAmount ?? 3);
        const chip = group.querySelector('#privacy-blur-amount-value');
        if (chip) chip.textContent = `${settings.blurAmount ?? 3}px`;
    }

    const opacity = group.querySelector('#setting-privacy-input-opacity');
    if (opacity) {
        opacity.value = String(settings.inputOpacity ?? 30);
        const chip = group.querySelector('#privacy-input-opacity-value');
        if (chip) chip.textContent = `${settings.inputOpacity ?? 30}%`;
    }

    const modeButtons = group.querySelectorAll('.privacy-mode-btn');
    modeButtons.forEach((btn) => {
        const active = btn.getAttribute('data-mode') === (settings.privacyMode || 'blur');
        btn.classList.toggle('active', active);
        btn.setAttribute('aria-pressed', active ? 'true' : 'false');
    });

    const lastNCount = group.querySelector('#setting-privacy-lastn-count');
    if (lastNCount) lastNCount.value = String(settings.unblurLastNCount ?? 3);
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

