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

            <div class="settings-section-title" style="margin-top: 32px;">About</div>
            <div class="settings-group">
                <div class="settings-toggle-row" style="cursor: default;">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Wassap Desktop</span>
                        <span class="settings-toggle-desc">
                            Multi-account WhatsApp client with isolated profile storage.
                        </span>
                    </div>
                    <span style="font-size: 12px; color: var(--text-secondary);">v0.4.0</span>
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
}
