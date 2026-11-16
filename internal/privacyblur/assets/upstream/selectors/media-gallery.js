/* ==========================================================================
   WhatsApp Privacy Blur - Media Gallery Selectors
   Rule key: blurMediaGallery
   ========================================================================== */

window.WA_BLUR_RULES.push({
  key: 'blurMediaGallery',
  className: 'wa-blur-media-gallery',
  label: 'Media Gallery',
  property: 'filter',
  blurMultiplier: 5,
  targets: [
    //  Conversation Media
    '[data-testid="media-video"]',
    '[data-testid="media-zoomable"]',
    'div | has:> [data-testid="media-gif"] | not-has:svg',
    '[data-testid="drawer-middle"] [data-animate-attach-media="true"] > div > div > div',
    'button[data-testid="webtp-powered-by"] | up:3 | find:> img',
    '[data-testid="media-editor-canvas"] | up:1'
  ],
  hoverHasTargets: [
    {
      // Story Media (No Caption)
      blurTarget: '[data-testid="status-player-uie"] [style*="position: absolute"] | has:img, video',
      hoverTrigger: '[data-testid="status-player-uie"] [style*="position: absolute"] | not-has:video | not-has:button'
    },
    {
      // Story Media (With Caption)
      blurTarget: '[data-testid="status-player-uie"] [style*="position: absolute"] | has:img, video',
      hoverTrigger: '[data-testid="status-player-uie"] [style*="position: absolute"] | has:button'
    }
  ]
});
