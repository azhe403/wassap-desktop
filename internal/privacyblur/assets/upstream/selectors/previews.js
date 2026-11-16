/* ==========================================================================
   WhatsApp Privacy Blur - Message Preview Selectors
   Rule key: blurPreviews
   ========================================================================== */

window.WA_BLUR_RULES.push({
  key: 'blurPreviews',
  className: 'wa-blur-previews',
  label: 'Last Message Preview',
  property: 'filter',
  blurMultiplier: 1,
  targets: [
    '[data-testid="cell-frame-container"]:not([data-testid="chat-info-drawer"] * ) div[data-testid="cell-frame-secondary"] > div:first-child',
    '[data-testid="message-yourself-row"] div[data-testid="cell-frame-secondary"] > div:first-child',
    '[data-testid="last-msg-status"] | closest:[data-testid="cell-frame-secondary"] | find:> div:first-child'
  ],
});
