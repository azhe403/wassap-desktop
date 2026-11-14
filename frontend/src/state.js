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
    currentView: 'account', // 'account' | 'settings' | 'profiles'
};
