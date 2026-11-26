/**
 * Renders the full-page Settings view inside #content-view.
 * Sub-navigation between sections is component-local; the last-selected
 * section survives re-renders within the session.
 *
 * @param {HTMLElement} container
 * @param {Object} handlers - { onCloseToTrayChange, onPrivacyBlurChange, privacyBlur }
 */
let settingsActivePanel = 'general';
let installedBuildDate = ''; // cached from GetVersion() so the comparison can fill in either order

function formatBuildUTC(iso) {
    if (!iso) return 'date unavailable';
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return 'date unavailable';
    const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
    const pad = (n) => String(n).padStart(2, '0');
    return `${pad(d.getUTCDate())} ${months[d.getUTCMonth()]} ${d.getUTCFullYear()} ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())} UTC`;
}

export function renderSettingsPage(container, handlers = {}) {
    container.innerHTML = `
        <div class="settings-page-view" role="region" aria-label="Settings">
            <div class="settings-page-header">
                <h2 class="settings-page-title">Settings</h2>
                <p class="settings-page-subtitle">Configure Wassap Desktop preferences</p>
            </div>

            <div class="settings-shell">
                <nav class="settings-nav" aria-label="Settings sections">
                    <button class="settings-nav-item active" type="button" data-panel="general" aria-current="page">
                        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.66 1.66 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.66 1.66 0 0 0-1.82-.33 1.66 1.66 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.66 1.66 0 0 0-1-1.51 1.66 1.66 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.66 1.66 0 0 0 .33-1.82 1.66 1.66 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.66 1.66 0 0 0 1.51-1 1.66 1.66 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.66 1.66 0 0 0 1.82.33h0a1.66 1.66 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.66 1.66 0 0 0 1 1.51h0a1.66 1.66 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.66 1.66 0 0 0-.33 1.82v0a1.66 1.66 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.66 1.66 0 0 0-1.51 1z"/></svg>
                        <span>General</span>
                    </button>
                    <button class="settings-nav-item" type="button" data-panel="privacy">
                        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 3l7 2.8v5.4c0 4.6-3 7.9-7 9.8-4-1.9-7-5.2-7-9.8V5.8L12 3z"/><path d="M9.5 12l2 2 3.5-3.5"/></svg>
                        <span>Privacy Blur</span>
                    </button>
                    <button class="settings-nav-item" type="button" data-panel="update">
                        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/></svg>
                        <span>Software Update</span>
                    </button>
                    <button class="settings-nav-item" type="button" data-panel="about">
                        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M12 16v-5"/><path d="M12 8h.01"/></svg>
                        <span>About</span>
                    </button>
                </nav>

                <div class="settings-panels">
                    <section class="settings-panel active" data-panel="general" aria-label="General settings">
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
                <label class="settings-toggle-row" for="setting-smart-ram-saver-page">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Smart RAM Saver</span>
                        <span class="settings-toggle-desc">
                            Automatically trims background session memory when system RAM usage is high.
                        </span>
                    </div>
                    <input 
                        id="setting-smart-ram-saver-page" 
                        class="settings-checkbox" 
                        type="checkbox" 
                    />
                </label>
            </div>

                    </section>

                    <section class="settings-panel" data-panel="privacy" aria-label="Privacy Blur settings">
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

                    </section>

                    <section class="settings-panel" data-panel="update" aria-label="Software Update settings">
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
                        <div class="update-compare" id="update-compare" hidden>
                            <div class="update-compare-row"><span class="update-compare-label">Installed</span><span id="update-compare-installed"></span></div>
                            <div class="update-compare-row"><span class="update-compare-label">Available</span><span id="update-compare-available"></span></div>
                        </div>
                        <div class="update-status-actions">
                            <button id="btn-update-check" class="update-action-btn update-action-secondary" type="button">Check for updates</button>
                            <button id="btn-update-action" class="update-action-btn" type="button" style="display:none;"></button>
                            <button id="btn-update-dismiss" class="update-link-btn" type="button" style="display:none;">Dismiss</button>
                        </div>
                    </div>
                </div>
            </div>

                    </section>

                    <section class="settings-panel" data-panel="about" aria-label="About">
            <div class="settings-section-title">About</div>
            <div class="settings-group">
                <div class="settings-toggle-row" style="cursor: default;">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Wassap Desktop</span>
                        <span class="settings-toggle-desc">
                            Multi-account WhatsApp client with isolated profile storage.
                        </span>
                    </div>
                    <span id="about-version" style="font-size: 12px; color: var(--text-secondary);">dev</span>
                    <span id="about-build-date" style="font-size: 12px; color: var(--text-secondary);"></span>
                </div>
            </div>

            <div class="settings-section-title" style="margin-top: 28px;">Third-Party Notices</div>
            <div class="settings-group">
                <div class="settings-toggle-row settings-notice-row" style="cursor: default;">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">WhatsApp Privacy Blur <span class="settings-license-chip">MIT</span></span>
                        <span class="settings-toggle-desc">
                            The engine behind Privacy Blur. by Nyihui — WhatsappPrivacyBlur-Extension, v3.2.0 @ 82cc20e, vendored unmodified.
                        </span>
                    </div>
                    <a class="settings-notice-link" href="https://github.com/Nyihui/WhatsappPrivacyBlur-Extension" data-open-url="https://github.com/Nyihui/WhatsappPrivacyBlur-Extension">Source</a>
                </div>

                <div class="settings-toggle-row settings-notice-row" style="cursor: default;">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Nunito <span class="settings-license-chip">SIL OFL 1.1</span></span>
                        <span class="settings-toggle-desc">
                            Typeface used by this app. Copyright 2016 The Nunito Project Authors.
                        </span>
                    </div>
                    <a class="settings-notice-link" href="https://fonts.google.com/specimen/Nunito" data-open-url="https://fonts.google.com/specimen/Nunito">Font</a>
                </div>

                <div class="settings-toggle-row settings-notice-row" style="cursor: default;">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">Wails <span class="settings-license-chip">MIT</span></span>
                        <span class="settings-toggle-desc">
                            Desktop application framework. Copyright (c) 2018-Present Lea Anthony.
                        </span>
                    </div>
                    <a class="settings-notice-link" href="https://wails.io" data-open-url="https://wails.io">wails.io</a>
                </div>

                <div class="settings-toggle-row settings-notice-row" style="cursor: default;">
                    <div class="settings-toggle-text">
                        <span class="settings-toggle-title">go-webview2 <span class="settings-license-chip">MIT</span></span>
                        <span class="settings-toggle-desc">
                            Go bindings for the Microsoft WebView2 control. Copyright (c) 2020 John Chadwick; portions Copyright (c) 2017 Serge Zaitsev.
                        </span>
                    </div>
                    <a class="settings-notice-link" href="https://github.com/wailsapp/go-webview2" data-open-url="https://github.com/wailsapp/go-webview2">Source</a>
                </div>

                <p class="settings-privacy-footnote" style="border-top: 1px solid var(--border-color);">
                    WhatsApp is a trademark of Meta Platforms, Inc. Wassap Desktop is not affiliated with, endorsed by, or sponsored by Meta or WhatsApp. It embeds WhatsApp Web for your own account.
                </p>

                <details class="settings-license-details">
                    <summary>Full license texts</summary>
                    <pre class="settings-license-text">MIT License — WhatsApp Privacy Blur (WhatsappPrivacyBlur-Extension)
Copyright (c) 2026 Nyihui

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

MIT License — Wails
Copyright (c) 2018-Present Lea Anthony

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

MIT License — go-webview2
Copyright (c) 2020 John Chadwick; portions Copyright (c) 2017 Serge Zaitsev

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

SIL Open Font License Version 1.1 - 26 February 2007
Copyright 2016 The Nunito Project Authors (contact@sansoxygen.com)
https://scripts.sil.org/OFL

PREAMBLE
The goals of the Open Font License (OFL) are to stimulate worldwide
development of collaborative font projects, to support the font creation
efforts of academic and linguistic communities, and to provide a free and
open framework in which fonts may be shared and improved in partnership
with others.

The OFL allows the licensed fonts to be used, studied, modified and
redistributed freely as long as they are not sold by themselves. The
fonts, including any derivative works, can be bundled, embedded,
redistributed and/or sold with any software provided that any reserved
names are not used by derivative works. The fonts and derivatives,
however, cannot be released under any other type of license. The
requirement for fonts to remain under this license does not apply
to any document created using the fonts or their derivatives.

DEFINITIONS
"Font Software" refers to the set of files released by the Copyright
Holder(s) under this license and clearly marked as such. This may
include source files, build scripts and documentation.

"Reserved Font Name" refers to any names specified as such after the
copyright statement(s).

"Original Version" refers to the collection of Font Software components as
distributed by the Copyright Holder(s).

"Modified Version" refers to any derivative made by adding to, deleting,
or substituting -- in part or in whole -- any of the components of the
Original Version, by changing formats or by porting the Font Software to a
new environment.

"Author" refers to any designer, engineer, programmer, technical
writer or other person who contributed to the Font Software.

PERMISSION &amp; CONDITIONS
Permission is hereby granted, free of charge, to any person obtaining
a copy of the Font Software, to use, study, copy, merge, embed, modify,
redistribute, and sell modified and unmodified copies of the Font
Software, subject to the following conditions:

1) Neither the Font Software nor any of its individual components,
in Original or Modified Versions, may be sold by itself.

2) Original or Modified Versions of the Font Software may be bundled,
redistributed and/or sold with any software, provided that each copy
contains the above copyright notice and this license. These can be
included either as stand-alone text files, human-readable headers or
in the appropriate machine-readable metadata fields within text or
binary files as long as those fields can be easily viewed by the user.

3) No Modified Version of the Font Software may use the Reserved Font
Name(s) unless explicit written permission is granted by the corresponding
Copyright Holder. This restriction only applies to the primary font name as
presented to the users.

4) The name(s) of the Copyright Holder(s) or the Author(s) of the Font
Software shall not be used to promote, endorse or advertise any
Modified Version, except to acknowledge the contribution(s) of the
Copyright Holder(s) and the Author(s) or with their explicit written
permission.

5) The Font Software, modified or unmodified, in part or in whole,
must be distributed entirely under this license, and must not be
distributed under any other license. The requirement for fonts to
remain under this license does not apply to any document created
using the Font Software.

TERMINATION
This license becomes null and void if any of the above conditions are
not met.

DISCLAIMER
THE FONT SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO ANY WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT
OF COPYRIGHT, PATENT, TRADEMARK, OR OTHER RIGHT. IN NO EVENT SHALL THE
COPYRIGHT HOLDER BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
INCLUDING ANY GENERAL, SPECIAL, INDIRECT, INCIDENTAL, OR CONSEQUENTIAL
DAMAGES, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
FROM, OUT OF THE USE OR INABILITY TO USE THE FONT SOFTWARE OR FROM
OTHER DEALINGS IN THE FONT SOFTWARE.</pre>
                </details>
            </div>
                    </section>
                </div>
            </div>
        </div>
    `;

    const checkbox = container.querySelector('#setting-close-to-tray-page');

    // Load persisted value asynchronously
    const api = window.go && window.go.app && window.go.app.App;
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

    const smartRAMCheckbox = container.querySelector('#setting-smart-ram-saver-page');
    if (api && api.GetSmartRAMSaver) {
        api.GetSmartRAMSaver().then((val) => {
            if (smartRAMCheckbox) smartRAMCheckbox.checked = !!val;
        }).catch((err) => {
            console.error('Failed to load smart RAM saver setting:', err);
        });
    }

    if (smartRAMCheckbox) {
        smartRAMCheckbox.addEventListener('change', async (e) => {
            const enabled = e.target.checked;
            if (handlers.onSmartRAMSaverChange) {
                const ok = await handlers.onSmartRAMSaverChange(enabled);
                if (ok === false) {
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
    const aboutBuildDate = container.querySelector('#about-build-date');
    if (api && api.GetVersion && aboutVersion) {
        api.GetVersion().then((v) => {
            aboutVersion.textContent = `v${v.version}${v.commit ? ' (' + v.commit.slice(0, 7) + ')' : ''}`;
            installedBuildDate = v.buildDate || '';
            if (aboutBuildDate) {
                aboutBuildDate.textContent = v.buildDate ? `build ${formatBuildUTC(v.buildDate)}` : '';
            }
            if (container.__updateStatus) {
                renderUpdateStatus(container, container.__updateStatus);
            }
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

    // --- External links: open in the system browser, never navigate the shell ---

    container.querySelectorAll('[data-open-url]').forEach((el) => {
        el.addEventListener('click', (e) => {
            e.preventDefault();
            const url = el.getAttribute('data-open-url');
            if (!url) return;
            if (window.runtime && window.runtime.BrowserOpenURL) {
                window.runtime.BrowserOpenURL(url);
            } else if (window.open) {
                window.open(url, '_blank', 'noopener');
            }
        });
    });

    // --- Sub-navigation between sections -------------------------------

    const navItems = container.querySelectorAll('.settings-nav-item');
    const panels = container.querySelectorAll('.settings-panel');

    function showPanel(name) {
        settingsActivePanel = name;
        navItems.forEach((item) => {
            const active = item.getAttribute('data-panel') === name;
            item.classList.toggle('active', active);
            if (active) {
                item.setAttribute('aria-current', 'page');
            } else {
                item.removeAttribute('aria-current');
            }
        });
        panels.forEach((panel) => {
            panel.classList.toggle('active', panel.getAttribute('data-panel') === name);
        });
    }

    navItems.forEach((item) => {
        item.addEventListener('click', () => {
            showPanel(item.getAttribute('data-panel'));
        });
    });

    // Re-entering the page preserves the section the user was last on.
    showPanel(settingsActivePanel);
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
    const compareEl = container.querySelector('#update-compare');
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
            text = 'This is a development build. Install a release from GitHub to enable updates.';
            break;
        case 'unknown':
            text = 'Not checked yet.';
            break;
        default:
            text = s.message || '';
    }

    textEl.textContent = text;
    textEl.classList.toggle('error', tone === 'error');

    if (compareEl) {
        const showCompare = (s.state === 'available' || s.state === 'downloaded') && !!s.latestVersion;
        compareEl.hidden = !showCompare;
        if (showCompare) {
            const installed = container.querySelector('#update-compare-installed');
            const available = container.querySelector('#update-compare-available');
            if (installed) {
                installed.textContent = `v${s.currentVersion || 'dev'} · ${formatBuildUTC(installedBuildDate)}`;
            }
            if (available) {
                available.textContent = `${s.latestVersion} · ${formatBuildUTC(s.latestBuildDate)}`;
            }
        }
    }

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

