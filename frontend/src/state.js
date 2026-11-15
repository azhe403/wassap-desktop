/**
 * Centralized frontend application state.
 */
export const state = {
    accounts: [],
    activeAccountId: null,
    switchingAccountId: null,
    // switching locks account switches while one is running, so a second
    // click cannot leave the sidebar pointing at a different account than
    // the one on screen.
    switching: false,
    openedAccountIds: new Set(),
    isLoading: true,
    error: null,
    resourceUsage: null, // last sample from GetResourceUsage (polled every 2s)
    currentView: 'account', // 'account' | 'settings' | 'profiles'
    updateBadge: false, // true when an update is available (or downloaded) and not dismissed
};
