/* ==========================================================================
   WhatsApp Privacy Blur - Chat Text Selectors
   Rule key: blurTextChats
   ========================================================================== */

window.WA_BLUR_RULES.push({
  key: 'blurTextChats',
  className: 'wa-blur-text',
  label: 'All Messages In Chat',
  property: 'filter',
  blurMultiplier: 1,
  targets: [
    '[data-testid="msg-container"] div | has:> span[data-testid*="selectable-text"]:not([data-testid="quoted-message"] *)',
    '[data-testid="recalled"] | up:2',
    'div[class*="copyable-text"] | up:1 | has:p',
    'div[data-testid*="selectable-text"] | up:1 | has:img',
    '[data-testid="group-notification-context-card-title"]',
    '[data-testid="group-notification-context-card-subtitle"]',
    '[data-testid="msg-notification-container"] div | has:> [data-testid="fmx-card-trust-signals"]',
    '[data-testid="msg-notification-container"] [data-testid*="subtype"]',
    '[data-testid="msg-notification-container"] [data-testid="pinned_message_system_message"]',
    '[data-testid="msg-notification-container"] > div:has(false)',
    '[role="button"][aria-label="Close"] | up:1 | find:> div:nth-child(2) | has:span',
    '[data-testid="media-caption"]',
    'span[data-testid="selectable-text"] | closest:div | has:[data-testid="link-preview-container"]',
    'div | has:> [data-testid="quoted-message"]',
    'div[data-testid="msg-notification-container"] | has:[data-testid="message-history-notice-message"]',

    //  Pinned Messages
    '[data-testid="conversation-panel-wrapper"] [data-testid="conversation-subheader"] > div:first-child > div:first-child > div:first-child > div:last-child',

    //  Group Descriptions
    '[data-testid="conversation-subheader"] > div > div > div:nth-child(2)',
    'span[data-testid*="group-notification-context-card-description"] | closest:div',
    'span[data-testid*="group-info-drawer-description-title-input-read-only"] | closest:div',
    'span[data-testid*="community-home-drawer-description-title-input-read-only"] | closest:div',

    //  Story Captions
    '[data-testid="status-player-uie"] span[dir="auto"]:not(button[data-testid="status-player-contact-name"] *):not([class*="html-span"] *)'
  ],
});
