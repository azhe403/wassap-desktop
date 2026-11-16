/* ==========================================================================
   WhatsApp Privacy Blur - Sticker + Transition Selectors
   Rule keys: blurStickers, noTransition
   ========================================================================== */

window.WA_BLUR_RULES.push(
  {
    key: 'blurStickers',
    className: 'wa-blur-stickers',
    label: 'Stickers',
    property: 'filter',
    blurMultiplier: 2,
    targets: [
      'span[data-testid="sticker-container"] | closest:div'
    ],
  },
  {
    key: 'noTransition',
    className: 'wa-no-transition',
    label: 'No Transition Delay',
    special: 'noTransition',
  }
);
