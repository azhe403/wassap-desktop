// Avatar initials and deterministic color palette for accounts

const PALETTE = [
    { bg: '#005c4b', fg: '#ffffff' }, // WhatsApp Emerald
    { bg: '#1f7a8c', fg: '#ffffff' }, // Ocean Teal
    { bg: '#2b2d42', fg: '#ffffff' }, // Slate Charcoal
    { bg: '#6b2d5c', fg: '#ffffff' }, // Muted Purple
    { bg: '#8d5b4c', fg: '#ffffff' }, // Earth Bronze
    { bg: '#3a5a40', fg: '#ffffff' }, // Forest
    { bg: '#0f4c5c', fg: '#ffffff' }, // Deep Cyan
    { bg: '#5c4d7d', fg: '#ffffff' }, // Dusty Indigo
];

/**
 * Extracts 1-2 uppercase characters from an account name.
 * e.g. "Personal Account" -> "PA", "Work" -> "W", "A" -> "A".
 */
export function getInitials(name) {
    if (!name || typeof name !== 'string') return '?';
    const words = name.trim().split(/\s+/).filter(Boolean);
    if (words.length === 0) return '?';
    if (words.length === 1) {
        return words[0].slice(0, 2).toUpperCase();
    }
    return (words[0][0] + words[words.length - 1][0]).toUpperCase();
}

/**
 * Computes a deterministic color from an account identifier or name.
 */
export function getAvatarColor(key) {
    if (!key) return PALETTE[0];
    let hash = 0;
    for (let i = 0; i < key.length; i++) {
        hash = (hash << 5) - hash + key.charCodeAt(i);
        hash |= 0;
    }
    const idx = Math.abs(hash) % PALETTE.length;
    return PALETTE[idx];
}
