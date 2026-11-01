package storage_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"wassap-desktop/internal/account"
	"wassap-desktop/internal/storage"
)

func TestJSONRepository_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "accounts.json")

	repo, err := storage.NewJSONAccountRepository(filePath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	accounts, err := repo.List()
	if err != nil {
		t.Fatalf("failed to list accounts: %v", err)
	}
	if len(accounts) != 0 {
		t.Fatalf("expected 0 accounts, got %d", len(accounts))
	}
}

func TestJSONRepository_SaveAndGet(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "accounts.json")

	repo, err := storage.NewJSONAccountRepository(filePath)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}

	acc1, err := account.NewAccount("Work")
	if err != nil {
		t.Fatal(err)
	}
	acc2, err := account.NewAccount("Personal")
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.Save(*acc1); err != nil {
		t.Fatalf("failed to save acc1: %v", err)
	}
	if err := repo.Save(*acc2); err != nil {
		t.Fatalf("failed to save acc2: %v", err)
	}

	// Verify Get
	got1, err := repo.Get(acc1.ID)
	if err != nil {
		t.Fatalf("failed to get acc1: %v", err)
	}
	if got1.Name != "Work" || got1.ID != acc1.ID {
		t.Fatalf("mismatched acc1: %+v", got1)
	}

	// Verify List preserves order
	list, err := repo.List()
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(list))
	}
	if list[0].ID != acc1.ID || list[1].ID != acc2.ID {
		t.Fatalf("order not preserved: [0]=%s, [1]=%s", list[0].ID, list[1].ID)
	}

	// Verify persistence across new repo instance on same file
	repo2, err := storage.NewJSONAccountRepository(filePath)
	if err != nil {
		t.Fatalf("failed to reload repo: %v", err)
	}
	reloaded, err := repo2.List()
	if err != nil {
		t.Fatalf("failed to list from reloaded repo: %v", err)
	}
	if len(reloaded) != 2 || reloaded[0].ID != acc1.ID || reloaded[1].ID != acc2.ID {
		t.Fatalf("reloaded data mismatch: %+v", reloaded)
	}
}

func TestJSONRepository_Update(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "accounts.json")

	repo, err := storage.NewJSONAccountRepository(filePath)
	if err != nil {
		t.Fatal(err)
	}

	acc1, _ := account.NewAccount("Work")
	acc2, _ := account.NewAccount("Personal")
	_ = repo.Save(*acc1)
	_ = repo.Save(*acc2)

	// Update acc1 name
	acc1.Name = "Renamed Work"
	acc1.UpdatedAt = time.Now().UTC()
	if err := repo.Save(*acc1); err != nil {
		t.Fatalf("failed to update acc1: %v", err)
	}

	got, err := repo.Get(acc1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Renamed Work" {
		t.Fatalf("expected Renamed Work, got %s", got.Name)
	}

	// Ordering must still be [acc1, acc2]
	list, _ := repo.List()
	if len(list) != 2 || list[0].ID != acc1.ID || list[1].ID != acc2.ID {
		t.Fatalf("list order changed after update: %+v", list)
	}
}

func TestJSONRepository_Delete(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "accounts.json")

	repo, err := storage.NewJSONAccountRepository(filePath)
	if err != nil {
		t.Fatal(err)
	}

	acc, _ := account.NewAccount("To Delete")
	_ = repo.Save(*acc)

	if err := repo.Delete(acc.ID); err != nil {
		t.Fatalf("failed to delete account: %v", err)
	}

	// Getting deleted account should return ErrAccountNotFound
	_, err = repo.Get(acc.ID)
	if !errors.Is(err, account.ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}

	// Deleting again should return ErrAccountNotFound
	err = repo.Delete(acc.ID)
	if !errors.Is(err, account.ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound on second delete, got %v", err)
	}
}

func TestJSONRepository_CorruptedJSON(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "accounts.json")

	corruptedContent := []byte("{ this is not valid json --- !!! }")
	if err := os.WriteFile(filePath, corruptedContent, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := storage.NewJSONAccountRepository(filePath)
	if err == nil {
		t.Fatal("expected error loading corrupted JSON, got nil")
	}
	if !errors.Is(err, account.ErrCorruptData) {
		t.Fatalf("expected ErrCorruptData, got: %v", err)
	}

	// Verify original corrupted file was NOT silently overwritten or wiped
	contentAfter, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(contentAfter) != string(corruptedContent) {
		t.Fatalf("file content was altered: %s", string(contentAfter))
	}
}

func TestJSONRepository_AutoCreateDirectory(t *testing.T) {
	tempDir := t.TempDir()
	deepPath := filepath.Join(tempDir, "nested", "sub", "dir", "accounts.json")

	repo, err := storage.NewJSONAccountRepository(deepPath)
	if err != nil {
		t.Fatalf("failed to create repo in deep path: %v", err)
	}

	acc, _ := account.NewAccount("Nested Test")
	if err := repo.Save(*acc); err != nil {
		t.Fatalf("failed to save in deep path: %v", err)
	}

	if _, err := os.Stat(deepPath); err != nil {
		t.Fatalf("expected file to exist at %s, got: %v", deepPath, err)
	}
}

func TestJSONRepository_ConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "accounts.json")

	repo, err := storage.NewJSONAccountRepository(filePath)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	workers := 20
	opsPerWorker := 10

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < opsPerWorker; j++ {
				name := fmt.Sprintf("Acc-%d-%d", workerID, j)
				acc, err := account.NewAccount(name)
				if err != nil {
					t.Errorf("NewAccount error: %v", err)
					return
				}
				if err := repo.Save(*acc); err != nil {
					t.Errorf("Save error: %v", err)
					return
				}
				if _, err := repo.List(); err != nil {
					t.Errorf("List error: %v", err)
					return
				}
				if _, err := repo.Get(acc.ID); err != nil {
					t.Errorf("Get error: %v", err)
					return
				}
			}
		}(i)
	}

	wg.Wait()

	list, err := repo.List()
	if err != nil {
		t.Fatal(err)
	}
	expectedCount := workers * opsPerWorker
	if len(list) != expectedCount {
		t.Fatalf("expected %d accounts, got %d", expectedCount, len(list))
	}
}
