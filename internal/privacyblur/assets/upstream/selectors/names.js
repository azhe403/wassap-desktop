/* ==========================================================================
   WhatsApp Privacy Blur - Name Selectors
   Rule key: blurNames
   ========================================================================== */

window.WA_BLUR_RULES.push({
  key: 'blurNames',
  className: 'wa-blur-names',
  label: "Group / User's Name",
  property: 'filter',
  blurMultiplier: 1,
  targets: [

    // EXPERIMENTAL
    '[role="gridcell"] [data-testid="cell-frame-label"]',
    '[role="gridcell"] [data-testid="cell-frame-title"]',
    '[data-testid="recent-search-item"] > :last-child',

    //  Me Tab Drawer Sidebar
    '[data-testid="me-tab-drawer"] [data-testid="drawer-title-body"]',
    '[data-testid="profile-drawer"] [data-testid*="pushname-input-read-only"] | up:1',
    '[data-testid="profile-drawer"] [data-testid="phone"] | up:1 | find:> div',
    //  Username
    '[data-testid="profile-drawer"] [data-testid="mentions-refreshed"] | up:1 | find:> div:nth-child(2)',
    '[data-testid="creating-username-info-drawer"] > div:nth-child(2) > div:nth-child(2) > div:first-child > div > div:nth-child(2)',
    //  Update 18-06-2026 - v2.3000.1047882213 || For both Username and Phone number
    '[data-testid="profile-drawer"] [data-testid="pushname-description"] ~ div [data-testid="container"] div | has:> span > svg | find:> div',

    //  Starred Messages
    '[data-testid="drawer-left"] span:first-child div | has:> div > [data-testid*="conv-msg-"] | find:> div:first-child > div:first-child',

    //  Status Player
    '[data-testid="status-player-contact-name"]',
    //  Channel Sidebar
    '[data-testid="newsletter-recommended-item"] > [data-testid="cell-frame-container"] > div > div:nth-child(2) > div:first-child',
    //  Conversation Panel - Messages
    'div | has:> span[data-testid="author"]:not([data-testid="quoted-message"] *)',
    '[data-testid="vcard-msg"] > div:nth-child(2)',
    //  Conversation Panel - Reactions
    'div | has:> [data-testid="reaction-count-header"] | find:> div:last-child [data-testid*="list-item-"] [data-testid="cell-frame-secondary"]',

    //  Right Sidebar - Chat Info Drawer
    'span[data-testid*="contact-info-subtitle"] | up:4',
    '[data-testid="chat-info-drawer"] | has:[data-testid*="contact-info-subtitle"] | find:[data-testid="li-block"] > div:first-child > div:last-child',
    '[data-testid="chat-info-drawer"] | has:[data-testid*="contact-info-subtitle"] | find:[data-testid="li-report-spam"] > div:first-child > div:last-child',
    'span[data-testid*="group-info-drawer-subject-input-read-only"] | closest:div',
    'span[data-testid*="community-home-subject-input-read-only"] | closest:div',
    '[data-testid="group_info_created_by_wrapper"]',
    //  Right Sidebar - Group Participants Info
    '[data-testid="group-info-participants-section"] [role="listitem"] [data-testid="cell-frame-container"] > div:nth-child(2) [data-testid="cell-frame-secondary"] > div:nth-child(2)',
    //  Right Sidebar - Common Group
    '[data-testid="section-common-groups"] | up:1 | find:[data-testid="cell-frame-container"] [data-testid="cell-frame-secondary"]',
    //  Right Sidebar - Links Gallery Messages
    '[data-testid="link-gallery-msg"] > div:first-child > div:first-child > span[dir="auto"]',
    //  Popup Members
    '[data-testid="contacts-modal"] [role="listitem"] [data-testid="cell-frame-container"] [data-testid="cell-frame-secondary"] > div:nth-child(2)',
  ],
  hoverHasTargets: [
    //Update 12-11-2026 || For Conversation Panel - Header
    {
      blurTarget: 'div[data-testid="conversation-info-header-content"] > div:first-child',
      hoverTrigger: 'button[data-testid="conversation-info-header-button"]'
    },
    {
      blurTarget: 'div[data-testid="conversation-info-header-content"] [data-testid="chat-subtitle"] | has:span',
      hoverTrigger: 'button[data-testid="conversation-info-header-button"]'
    }
  ]
});
