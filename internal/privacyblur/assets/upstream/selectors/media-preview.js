/* ==========================================================================
   WhatsApp Privacy Blur - Media Preview Selectors
   Rule key: blurMediaPreview
   ========================================================================== */

window.WA_BLUR_RULES.push({
  key: 'blurMediaPreview',
  className: 'wa-blur-media-preview',
  label: 'Media Preview',
  property: 'filter',
  blurMultiplier: 1,
  targets: [
    //  Conversation Panel
    '[data-testid="media-url-provider"]:not([data-testid="media-hub-thumb"] *, [data-testid="quoted-message"] *)',
    '[data-testid="link-preview-container"]',
    '[data-testid="document-thumb"]',
    // '[data-testid="quoted-message"] > div > div:last-child [style*="background-image: url"] | up:1',
    // //  Media List on Fullscreen View
    '[role="list"] [role="listitem"] [role="tab"] img | closest:div',
    // //  Media Hub Dialog
    '[data-testid="media-hub-modal"] [data-testid="media-hub-thumb"]',
    '[data-testid="media-hub-modal"] [data-focusid="media-hub-row-item"]',
    '[data-testid="status-thumbnail"] > div',
  ],
  hoverParentTargets: [
    {
      hoverParent: '[data-testid="image-thumb-gif"]',
      child: 'video',
    },
    {
      hoverParent: '[data-testid="video-content"]',
      child: '> div[style*="background-image: url"]',
    },
    {
      hoverParent: '[data-testid="image-thumb"]',
      child: '> img',
    },
    {
      hoverParent: '[data-testid="media-canvas"] > div',
      child: '> button > div[style*="background-image: url"]',
    },
  ],
});
