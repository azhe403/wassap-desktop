// Accessible modal dialogs with keyboard trapping and focus management

let activeModalCleanup = null;

/**
 * Shows the "Add Account" dialog.
 */
export function showAddAccountModal({ onConfirm, onCancel, onVisibilityChange }) {
    closeActiveModal();
    if (onVisibilityChange) onVisibilityChange(false);

    const backdrop = document.createElement('div');
    backdrop.className = 'modal-backdrop';
    backdrop.setAttribute('role', 'dialog');
    backdrop.setAttribute('aria-modal', 'true');
    backdrop.setAttribute('aria-labelledby', 'modal-add-title');

    backdrop.innerHTML = `
        <div class="modal-box">
            <div class="modal-header">
                <h3 id="modal-add-title" class="modal-title">Add WhatsApp Account</h3>
                <button class="modal-close-btn" id="modal-btn-x" aria-label="Close">✕</button>
            </div>
            <form id="modal-form">
                <div class="modal-body">
                    <label class="modal-input-label" for="account-name-input">Account Name</label>
                    <input 
                        id="account-name-input" 
                        class="modal-input" 
                        type="text" 
                        placeholder="e.g. Personal, Work, Business" 
                        maxlength="100" 
                        autocomplete="off"
                        required
                    />
                    <div id="modal-error" class="modal-error-msg" style="display: none;"></div>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn-secondary" id="modal-btn-cancel">Cancel</button>
                    <button type="submit" class="btn-primary" id="modal-btn-submit">Create Account</button>
                </div>
            </form>
        </div>
    `;

    document.body.appendChild(backdrop);
    const input = backdrop.querySelector('#account-name-input');
    const form = backdrop.querySelector('#modal-form');
    const errDiv = backdrop.querySelector('#modal-error');
    input.focus();

    const cleanup = () => {
        document.removeEventListener('keydown', handleKeyDown);
        if (backdrop.parentNode) {
            backdrop.parentNode.removeChild(backdrop);
        }
        activeModalCleanup = null;
        if (onVisibilityChange) onVisibilityChange(true);
    };

    activeModalCleanup = cleanup;

    const handleKeyDown = (e) => {
        if (e.key === 'Escape') {
            e.preventDefault();
            cleanup();
            if (onCancel) onCancel();
        }
    };
    document.addEventListener('keydown', handleKeyDown);

    backdrop.querySelector('#modal-btn-x').addEventListener('click', () => {
        cleanup();
        if (onCancel) onCancel();
    });
    backdrop.querySelector('#modal-btn-cancel').addEventListener('click', () => {
        cleanup();
        if (onCancel) onCancel();
    });

    form.addEventListener('submit', (e) => {
        e.preventDefault();
        const val = input.value.trim();
        if (!val) {
            errDiv.innerText = 'Account name cannot be empty.';
            errDiv.style.display = 'block';
            input.focus();
            return;
        }
        cleanup();
        if (onConfirm) onConfirm(val);
    });
}

/**
 * Shows the "Rename Account" dialog.
 */
export function showRenameModal({ currentName, onConfirm, onCancel, onVisibilityChange }) {
    closeActiveModal();
    if (onVisibilityChange) onVisibilityChange(false);

    const backdrop = document.createElement('div');
    backdrop.className = 'modal-backdrop';
    backdrop.setAttribute('role', 'dialog');
    backdrop.setAttribute('aria-modal', 'true');
    backdrop.setAttribute('aria-labelledby', 'modal-rename-title');

    backdrop.innerHTML = `
        <div class="modal-box">
            <div class="modal-header">
                <h3 id="modal-rename-title" class="modal-title">Rename Account</h3>
                <button class="modal-close-btn" id="modal-btn-x" aria-label="Close">✕</button>
            </div>
            <form id="modal-form">
                <div class="modal-body">
                    <label class="modal-input-label" for="account-name-input">New Account Name</label>
                    <input 
                        id="account-name-input" 
                        class="modal-input" 
                        type="text" 
                        value="${escapeAttr(currentName)}" 
                        maxlength="100" 
                        autocomplete="off"
                        required
                    />
                    <div id="modal-error" class="modal-error-msg" style="display: none;"></div>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn-secondary" id="modal-btn-cancel">Cancel</button>
                    <button type="submit" class="btn-primary" id="modal-btn-submit">Save Changes</button>
                </div>
            </form>
        </div>
    `;

    document.body.appendChild(backdrop);
    const input = backdrop.querySelector('#account-name-input');
    const form = backdrop.querySelector('#modal-form');
    const errDiv = backdrop.querySelector('#modal-error');
    input.focus();
    input.select();

    const cleanup = () => {
        document.removeEventListener('keydown', handleKeyDown);
        if (backdrop.parentNode) {
            backdrop.parentNode.removeChild(backdrop);
        }
        activeModalCleanup = null;
        if (onVisibilityChange) onVisibilityChange(true);
    };

    activeModalCleanup = cleanup;

    const handleKeyDown = (e) => {
        if (e.key === 'Escape') {
            e.preventDefault();
            cleanup();
            if (onCancel) onCancel();
        }
    };
    document.addEventListener('keydown', handleKeyDown);

    backdrop.querySelector('#modal-btn-x').addEventListener('click', () => {
        cleanup();
        if (onCancel) onCancel();
    });
    backdrop.querySelector('#modal-btn-cancel').addEventListener('click', () => {
        cleanup();
        if (onCancel) onCancel();
    });

    form.addEventListener('submit', (e) => {
        e.preventDefault();
        const val = input.value.trim();
        if (!val) {
            errDiv.innerText = 'Account name cannot be empty.';
            errDiv.style.display = 'block';
            input.focus();
            return;
        }
        cleanup();
        if (onConfirm) onConfirm(val);
    });
}

/**
 * Shows a confirmation dialog (e.g. for account removal).
 */
export function showConfirmModal({ title, message, confirmText = 'Confirm', isDanger = false, onConfirm, onCancel, onVisibilityChange }) {
    closeActiveModal();
    if (onVisibilityChange) onVisibilityChange(false);

    const backdrop = document.createElement('div');
    backdrop.className = 'modal-backdrop';
    backdrop.setAttribute('role', 'dialog');
    backdrop.setAttribute('aria-modal', 'true');

    backdrop.innerHTML = `
        <div class="modal-box">
            <div class="modal-header">
                <h3 class="modal-title">${escapeHTML(title)}</h3>
                <button class="modal-close-btn" id="modal-btn-x" aria-label="Close">✕</button>
            </div>
            <div class="modal-body">
                <p style="font-size: 14px; line-height: 1.5; color: var(--text-secondary);">${escapeHTML(message)}</p>
            </div>
            <div class="modal-footer">
                <button type="button" class="btn-secondary" id="modal-btn-cancel">Cancel</button>
                <button type="button" class="${isDanger ? 'btn-danger' : 'btn-primary'}" id="modal-btn-confirm">${escapeHTML(confirmText)}</button>
            </div>
        </div>
    `;

    document.body.appendChild(backdrop);
    const confirmBtn = backdrop.querySelector('#modal-btn-confirm');
    confirmBtn.focus();

    const cleanup = () => {
        document.removeEventListener('keydown', handleKeyDown);
        if (backdrop.parentNode) {
            backdrop.parentNode.removeChild(backdrop);
        }
        activeModalCleanup = null;
        if (onVisibilityChange) onVisibilityChange(true);
    };

    activeModalCleanup = cleanup;

    const handleKeyDown = (e) => {
        if (e.key === 'Escape') {
            e.preventDefault();
            cleanup();
            if (onCancel) onCancel();
        }
    };
    document.addEventListener('keydown', handleKeyDown);

    backdrop.querySelector('#modal-btn-x').addEventListener('click', () => {
        cleanup();
        if (onCancel) onCancel();
    });
    backdrop.querySelector('#modal-btn-cancel').addEventListener('click', () => {
        cleanup();
        if (onCancel) onCancel();
    });
    confirmBtn.addEventListener('click', () => {
        cleanup();
        if (onConfirm) onConfirm();
    });
}

export function closeActiveModal() {
    if (activeModalCleanup) {
        activeModalCleanup();
        activeModalCleanup = null;
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
