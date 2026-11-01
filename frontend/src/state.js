/**
 * Centralized frontend application state.
 */
export const state = {
    accounts: [],
    activeAccountId: null,
    switchingAccountId: null,
    openedAccountIds: new Set(),
    isLoading: true,
    error: null,
};
