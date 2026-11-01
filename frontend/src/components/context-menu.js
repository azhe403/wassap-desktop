// Custom desktop right-click context menu

let activeMenuCleanup = null;

/**
 * Shows the custom context menu for an account.
 *
 * @param {number} x
 * @param {number} y
 * @param {Object} account
 * @param {Object} actions { onRename, onRemove }
 */
export function showContextMenu(x, y, account, actions) {
    closeContextMenu();

    const menu = document.createElement('div');
    menu.className = 'context-menu';
    menu.setAttribute('role', 'menu');
    menu.setAttribute('aria-label', `Actions for ${account.name}`);

    menu.innerHTML = `
        <div class="context-menu-item" role="menuitem" tabindex="0" id="ctx-rename">
            <span>✏️</span>
            <span>Rename</span>
        </div>
        <div class="context-menu-item danger" role="menuitem" tabindex="0" id="ctx-remove">
            <span>🗑️</span>
            <span>Remove</span>
        </div>
    `;

    document.body.appendChild(menu);

    // Adjust position to stay within viewport
    const menuRect = menu.getBoundingClientRect();
    let left = x;
    let top = y;
    if (left + menuRect.width > window.innerWidth) {
        left = window.innerWidth - menuRect.width - 8;
    }
    if (top + menuRect.height > window.innerHeight) {
        top = window.innerHeight - menuRect.height - 8;
    }
    menu.style.left = `${left}px`;
    menu.style.top = `${top}px`;

    const cleanup = () => {
        document.removeEventListener('click', handleOutsideClick);
        document.removeEventListener('keydown', handleKeyDown);
        if (menu.parentNode) {
            menu.parentNode.removeChild(menu);
        }
        activeMenuCleanup = null;
    };
    activeMenuCleanup = cleanup;

    const handleOutsideClick = (e) => {
        if (!menu.contains(e.target)) {
            cleanup();
        }
    };
    const handleKeyDown = (e) => {
        if (e.key === 'Escape') {
            e.preventDefault();
            cleanup();
        }
    };

    setTimeout(() => {
        document.addEventListener('click', handleOutsideClick);
        document.addEventListener('keydown', handleKeyDown);
    }, 10);

    menu.querySelector('#ctx-rename').addEventListener('click', (e) => {
        e.stopPropagation();
        cleanup();
        if (actions.onRename) actions.onRename(account);
    });

    menu.querySelector('#ctx-remove').addEventListener('click', (e) => {
        e.stopPropagation();
        cleanup();
        if (actions.onRemove) actions.onRemove(account);
    });
}

export function closeContextMenu() {
    if (activeMenuCleanup) {
        activeMenuCleanup();
        activeMenuCleanup = null;
    }
}
