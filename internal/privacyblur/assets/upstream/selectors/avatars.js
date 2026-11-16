/* ==========================================================================
   WhatsApp Privacy Blur - Avatar Selectors
   Rule key: blurAvatars
   ========================================================================== */

window.WA_BLUR_RULES.push(
  {
    key: 'blurAvatars',
    className: 'wa-blur-avatars',
    label: 'Profile Pictures',
    property: 'filter',
    blurMultiplier: 1,
    targets: [
      //  Main Chat List Sidebar
      'div | has:> button > [data-testid="navbar-item-me-tab-photo"]',
      '[data-testid*="list-item-"] [data-testid="message-yourself-row"] > div:first-child div | has:> img',
      '[data-testid*="list-item-"] [data-testid="cell-frame-container"] > div:first-child div | has:> img',
      //  Update 18-06-2026 - v2.3000.1047882213
      '[data-testid="chatlist-header"] [data-testid="navbar-footer-section"] div | has:> img',
      //  Update 15-11-2026 - v2.3000.1054643268
      '[data-testid*="list-item-"] [data-testid="message-yourself-row"] > div:first-child div | has:> svg',
      '[data-testid*="list-item-"] [data-testid="cell-frame-container"] > div:first-child div | has:> svg',
      // 'div | has:> svg foreignObject [style*="--x-color: var(--WDS-components-profile-photo-content"] | find::not([data-testid="conversation-header-avatar"] *)',

      //  Recent Search
      // '[data-testid="recent-search-item"] > div:first-child | not-has:[data-testid="default-contact-refreshed"] | not-has:[data-testid="default-group-refreshed"]',
      '[data-testid="recent-search-item"] > div:first-child',

      //  Me Tab Drawer Sidebar
      '[data-testid="me-tab-drawer"] [data-testid="menu-controller-focus-receiver"] img[src*="https://media"]',
      //  Update 18-06-2026 - v2.3000.1047882213
      '[data-testid="me-tab-drawer"] [data-testid="menu-controller-focus-receiver"] button | has:svg image',
      //  Update 02-08-2026 - v2.3000.1044305114
      '[data-testid="profile-drawer"] [data-testid="profile-pic-picker"]',

      //  Calls Tab Drawer
      // '[data-testid="calls-tab-drawer"] [data-testid="calls-drawer"] [id="pane-side"] [data-testid*="list-item-"] [data-testid*="chatlist-message-"] > div:first-child | not-has:[data-testid="default-contact-refreshed"] | not-has:[data-testid="default-group-refreshed"]',
      '[data-testid="calls-tab-drawer"] [data-testid="calls-drawer"] [id="pane-side"] [data-testid*="list-item-"] [data-testid*="chatlist-message-"] > div:first-child',

      //  Status Sidebar
      '[data-testid="status-tab-drawer"] [data-testid="status-header-add-status"] img[src*="https://media"]',
      //  Update 02-07-2026 || Self-Profile status locator
      '[data-testid="status-header"] div | has:> img',

      //  Status Player
      '[data-testid="status-player-uie"] img[src*="https"]:not([data-testid="sticker-item"] *, [data-testid="status-emoji-bar"] *, [style*="position: absolute"] *)',

      //  Conversation Panel - Header
      '[data-testid="conversation-header"] [data-testid="subgroup-switcher-button"] div | has:> div > img',

      //  Conversation Panel - Messages
      'div[data-testid="group-chat-profile-picture"]',
      '[data-testid="group-notification-context-card-photo"]',
      '[data-testid="conversation-panel-messages"] [data-testid="vcard-msg"] img',
      'div | has:> div > div > div > [data-testid="ptt-status"] | find:> div:first-child',

      //  Conversation Panel - Reactions
      'div:has( > [data-testid="reaction-count-header"]) > div:last-child [data-testid*="list-item-"] [data-testid="reactions-details-cell-me"] > div:first-child div | has:> img',
      'div:has( > [data-testid="reaction-count-header"]) > div:last-child [data-testid*="list-item-"] [data-testid="reactions-details-cell-not-me"] > div:first-child div | has:> img',

      //  Right Sidebar - Chat Info Drawer
      '[data-testid="chat-info-drawer"] > :nth-child(2) [class*="html-span"] | has:img',
      //  Update 18-06-2026 - v2.3000.1047882213
      '[data-testid="chat-info-drawer"] > :nth-child(2) [class*="html-span"] | has:svg image',

      // '[data-testid="chat-info-drawer"] [tabindex="-1"] > button[data-testid="group-pic-picker"] | up:2',
      '[data-testid="chat-info-drawer"] div | has:> [tabindex="-1"] > button[data-testid="group-pic-picker"] | not-has:[data-testid="default-group-refreshed"] *',

      //  Right Sidebar - Group's Media/Docs/Links
      '[data-testid="link-gallery-msg"] > div:first-child > div:first-child > div:first-child',

      //  Media Viewer
      '[data-testid="media-viewer-modal"] [data-testid="cell-frame-container"] img',

      //  Profile Picture Fullscreen Overlay
      '[class*="overlay"] [data-testid="cell-frame-container"] img',
      //  Update 18-06-2026 || x1akjpcp = background-color: rgba(var(--WDS-background-wash-plain-RGB),.96);
      '[class*="x1akjpcp"] [data-testid="cell-frame-container"] img',
    ],
    hoverParentTargets: [
      //  Update 02-08-2026 - v2.3000.1044305114 || Profile picture that has story rings.
      {
        hoverParent: '[data-testid*="list-item-"] [data-testid="message-yourself-row"] > div:first-child div:has( > svg image)',
        child: 'image',
      },
      {
        hoverParent: '[data-testid*="list-item-"] [data-testid="cell-frame-container"] > div:first-child div:has( > svg image)',
        child: 'image',
      },
    ],
    hoverHasTargets: [
      //Update 12-11-2026 || For Conversation Panel - Header
      {
        blurTarget: '[data-testid="conversation-header"] > div:first-child > div:first-child',
        hoverTrigger: 'button[data-testid="conversation-info-header-button"]'
      }
    ]
  },
  {
    key: 'blurAvatars',
    className: 'wa-blur-avatars',
    label: 'Profile Pictures',
    property: 'filter',
    blurMultiplier: 5,
    targets: [
      '[class*="overlay"] [dir="ltr"] img | closest:div',
      '[class*="overlay"] [dir="rtl"] img | closest:div',
      //  Update 18-06-2026 || x1akjpcp = background-color: rgba(var(--WDS-background-wash-plain-RGB),.96);
      '[class*="x1akjpcp"] [dir="ltr"] img | closest:div',
      '[class*="x1akjpcp"] [dir="rtl"] img | closest:div',
    ],
  }
);
