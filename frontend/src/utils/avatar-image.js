/**
 * Shared helpers for rendering cached WhatsApp profile photos.
 * Falls back to the initials markup used before avatar sync existed.
 */

function escapeAttr(value) {
    return String(value)
        .replace(/&/g, '&amp;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');
}

/**
 * @param {string} id account id
 * @param {Object} avatars id -> data URI map
 * @param {string} fallbackHTML initials markup when no photo is cached
 * @param {string} imgClass CSS class for the <img>
 * @returns {string}
 */
export function avatarImageHTML(id, avatars, fallbackHTML, imgClass = 'account-avatar-img') {
    const uri = avatars && avatars[id];
    if (typeof uri !== 'string' || !uri.startsWith('data:image/')) return fallbackHTML;
    return `<img class="${imgClass}" src="${escapeAttr(uri)}" alt="" aria-hidden="true" />`;
}

/**
 * Fetch cached avatars from the Wails binding. Failures are silent so the
 * initials rendering stays intact.
 * @param {Object} api window.go.app.App
 * @param {string[]} ids account ids
 * @returns {Promise<Object>}
 */
export async function fetchAvatars(api, ids) {
    const result = {};
    if (!api || !api.GetAccountAvatar || !Array.isArray(ids)) return result;
    await Promise.all(ids.map(async (id) => {
        try {
            const uri = await api.GetAccountAvatar(id);
            if (typeof uri === 'string' && uri.startsWith('data:image/')) result[id] = uri;
        } catch (e) {
            // no avatar cached, keep initials
        }
    }));
    return result;
}
