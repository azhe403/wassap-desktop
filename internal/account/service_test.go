package account_test

import (
	"errors"
	"testing"

	"wassap-desktop/internal/account"
	"wassap-desktop/internal/browser"
)

// mockRepository implements account.Repository in-memory for testing.
type mockRepository struct {
	accounts map[string]account.Account
	order    []string
	saveErr  error
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		accounts: make(map[string]account.Account),
	}
}

func (m *mockRepository) List() ([]account.Account, error) {
	out := make([]account.Account, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, m.accounts[id])
	}
	return out, nil
}

func (m *mockRepository) Get(id string) (*account.Account, error) {
	acc, ok := m.accounts[id]
	if !ok {
		return nil, account.ErrAccountNotFound
	}
	return &acc, nil
}

func (m *mockRepository) Save(acc account.Account) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	if _, exists := m.accounts[acc.ID]; !exists {
		m.order = append(m.order, acc.ID)
	}
	m.accounts[acc.ID] = acc
	return nil
}

func (m *mockRepository) Delete(id string) error {
	if _, ok := m.accounts[id]; !ok {
		return account.ErrAccountNotFound
	}
	delete(m.accounts, id)
	newOrder := make([]string, 0, len(m.order)-1)
	for _, item := range m.order {
		if item != id {
			newOrder = append(newOrder, item)
		}
	}
	m.order = newOrder
	return nil
}

// mockProfileManager implements browser.ManagerInterface for testing.
type mockProfileManager struct {
	created    map[string]bool
	deleted    map[string]bool
	active     map[string]bool
	createErr  error
	deleteErr  error
}

func newMockProfileManager() *mockProfileManager {
	return &mockProfileManager{
		created: make(map[string]bool),
		deleted: make(map[string]bool),
		active:  make(map[string]bool),
	}
}

func (m *mockProfileManager) Create(id string) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created[id] = true
	return nil
}

func (m *mockProfileManager) Open(id string) error {
	m.active[id] = true
	return nil
}

func (m *mockProfileManager) Close(id string) error {
	delete(m.active, id)
	return nil
}

func (m *mockProfileManager) Delete(id string) error {
	if m.active[id] {
		return browser.ErrProfileOpen
	}
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.deleted[id] = true
	delete(m.created, id)
	return nil
}

func TestService_CreateSuccess(t *testing.T) {
	repo := newMockRepository()
	pm := newMockProfileManager()
	svc := account.NewService(repo, pm)

	acc, err := svc.Create("Finance")
	if err != nil {
		t.Fatalf("unexpected error creating account: %v", err)
	}

	if acc.Name != "Finance" {
		t.Fatalf("expected Finance, got %s", acc.Name)
	}
	if !pm.created[acc.ID] {
		t.Fatalf("expected profile %s to be created in profile manager", acc.ID)
	}

	got, err := repo.Get(acc.ID)
	if err != nil {
		t.Fatalf("expected account to be persisted in repo: %v", err)
	}
	if got.ID != acc.ID {
		t.Fatalf("persisted ID mismatch")
	}
}

func TestService_CreateInvalidName(t *testing.T) {
	repo := newMockRepository()
	pm := newMockProfileManager()
	svc := account.NewService(repo, pm)

	_, err := svc.Create("   ")
	if !errors.Is(err, account.ErrInvalidAccountName) {
		t.Fatalf("expected ErrInvalidAccountName, got %v", err)
	}
	if len(pm.created) != 0 {
		t.Fatalf("expected no profile created on invalid name")
	}
}

func TestService_CreateProfileFailure(t *testing.T) {
	repo := newMockRepository()
	pm := newMockProfileManager()
	pm.createErr = errors.New("disk full")
	svc := account.NewService(repo, pm)

	_, err := svc.Create("Support")
	if err == nil {
		t.Fatal("expected error when profile creation fails, got nil")
	}
	if !errors.Is(err, account.ErrProfileCreationFailed) {
		t.Fatalf("expected ErrProfileCreationFailed, got %v", err)
	}
	if len(repo.accounts) != 0 {
		t.Fatalf("expected no metadata saved when profile creation fails")
	}
}

func TestService_CreatePersistenceFailure_Rollback(t *testing.T) {
	repo := newMockRepository()
	repo.saveErr = errors.New("database locked")
	pm := newMockProfileManager()
	svc := account.NewService(repo, pm)

	_, err := svc.Create("Engineering")
	if err == nil {
		t.Fatal("expected error when repo save fails, got nil")
	}

	// Verify rollback: profile should have been created then immediately deleted
	if len(pm.created) != 0 {
		t.Fatalf("expected created profile to be rolled back, but still present")
	}
	if len(pm.deleted) != 1 {
		t.Fatalf("expected 1 profile deletion during rollback, got %d", len(pm.deleted))
	}
}

func TestService_ListAndGet(t *testing.T) {
	repo := newMockRepository()
	pm := newMockProfileManager()
	svc := account.NewService(repo, pm)

	a1, _ := svc.Create("A1")
	a2, _ := svc.Create("A2")

	list, err := svc.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != a1.ID || list[1].ID != a2.ID {
		t.Fatalf("unexpected list: %+v", list)
	}

	got, err := svc.Get(a1.ID)
	if err != nil || got.Name != "A1" {
		t.Fatalf("unexpected Get result: %+v, err: %v", got, err)
	}

	_, err = svc.Get("nonexistent")
	if !errors.Is(err, account.ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestService_Rename(t *testing.T) {
	repo := newMockRepository()
	pm := newMockProfileManager()
	svc := account.NewService(repo, pm)

	acc, _ := svc.Create("Original Name")
	oldUpdatedAt := acc.UpdatedAt

	if err := svc.Rename(acc.ID, "New Name"); err != nil {
		t.Fatalf("failed to rename: %v", err)
	}

	updated, err := svc.Get(acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "New Name" {
		t.Fatalf("expected New Name, got %s", updated.Name)
	}
	if updated.ID != acc.ID {
		t.Fatalf("ID must not change during rename")
	}
	if !updated.UpdatedAt.After(oldUpdatedAt) && !updated.UpdatedAt.Equal(oldUpdatedAt) {
		t.Fatalf("UpdatedAt should be updated")
	}

	// Rename unknown account
	if err := svc.Rename("unknown", "Valid Name"); !errors.Is(err, account.ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound for unknown account, got %v", err)
	}

	// Rename with invalid name
	if err := svc.Rename(acc.ID, ""); !errors.Is(err, account.ErrInvalidAccountName) {
		t.Fatalf("expected ErrInvalidAccountName, got %v", err)
	}
}

func TestService_RemoveSuccess(t *testing.T) {
	repo := newMockRepository()
	pm := newMockProfileManager()
	svc := account.NewService(repo, pm)

	acc, _ := svc.Create("To Delete")

	if err := svc.Remove(acc.ID); err != nil {
		t.Fatalf("failed to remove account: %v", err)
	}

	// Profile must be deleted
	if !pm.deleted[acc.ID] {
		t.Fatalf("expected profile to be deleted in profile manager")
	}

	// Account must be removed from repo
	_, err := repo.Get(acc.ID)
	if !errors.Is(err, account.ErrAccountNotFound) {
		t.Fatalf("expected account to be deleted from repo")
	}
}

func TestService_RemoveActiveProfile_Refused(t *testing.T) {
	repo := newMockRepository()
	pm := newMockProfileManager()
	svc := account.NewService(repo, pm)

	acc, _ := svc.Create("Active Session")
	_ = pm.Open(acc.ID) // mark profile active

	err := svc.Remove(acc.ID)
	if err == nil {
		t.Fatal("expected error removing active profile, got nil")
	}
	if !errors.Is(err, account.ErrAccountBusy) {
		t.Fatalf("expected ErrAccountBusy, got %v", err)
	}

	// Verify profile and metadata are preserved
	if pm.deleted[acc.ID] {
		t.Fatalf("active profile must NOT be deleted")
	}
	if _, err := repo.Get(acc.ID); err != nil {
		t.Fatalf("account metadata must NOT be removed when profile is active")
	}
}

func TestService_RemoveProfileDeletionFailure(t *testing.T) {
	repo := newMockRepository()
	pm := newMockProfileManager()
	pm.deleteErr = errors.New("permission denied")
	svc := account.NewService(repo, pm)

	acc, _ := svc.Create("Keep On Failure")

	err := svc.Remove(acc.ID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, account.ErrProfileDeletionFailed) {
		t.Fatalf("expected ErrProfileDeletionFailed, got %v", err)
	}

	// Metadata must remain intact
	if _, err := repo.Get(acc.ID); err != nil {
		t.Fatalf("metadata must be preserved when profile deletion fails")
	}
}
