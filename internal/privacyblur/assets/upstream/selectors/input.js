/* ==========================================================================
   WhatsApp Privacy Blur - Text Input Selectors
   Rule key: blurInput
   ========================================================================== */

window.WA_BLUR_RULES.push({
  key: 'blurInput',
  className: 'wa-blur-input',
  label: 'Text Input',
  property: 'opacity',
  targets: [
    'div[class*="lexical-rich-text-input"] | up:1',

    //  Update 18/06/2026
    'div | has:> div > [data-lexical-editor="true"]',

    //  Search bar
    '[data-testid="chat-list-search-container"]',
  ],
});
