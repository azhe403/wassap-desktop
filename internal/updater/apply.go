package updater

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

// CopyStaged copies the staged update next to the running executable as
// <exeBase>.new-<pid> and re-verifies its digest. Copying next to the exe
// guarantees a same-volume rename for the swap. On any failure the partial
// copy is removed and the original exe is never touched.
func CopyStaged(stagedPath, exePath, wantSHA256 string) (string, error) {
	dir := filepath.Dir(exePath)
	base := filepath.Base(exePath)
	newPath := filepath.Join(dir, base+".new-"+strconv.Itoa(os.Getpid()))

	if err := copyFile(stagedPath, newPath); err != nil {
		_ = os.Remove(newPath)
		return "", fmt.Errorf("updater: stage copy failed: %w", err)
	}
	if err := VerifyFileSHA256(newPath, wantSHA256); err != nil {
		_ = os.Remove(newPath)
		return "", err
	}
	return newPath, nil
}

// Swap replaces exePath with newPath using two renames with no copy between
// them, so the exe is missing only for the instant between metadata
// operations. On failure of the second rename the original binary is
// restored. The rename function is a test seam; passing os.Rename is the
// production behavior.
func Swap(exePath, newPath string, rename func(a, b string) error) error {
	oldPath := exePath + ".old-" + strconv.Itoa(os.Getpid())
	if err := rename(exePath, oldPath); err != nil {
		// newPath stays in place for the caller to remove; exe is untouched.
		return fmt.Errorf("updater: cannot rename %s aside (folder writable?): %w", exePath, err)
	}
	if err := rename(newPath, exePath); err != nil {
		if rbErr := rename(oldPath, exePath); rbErr != nil {
			return fmt.Errorf("updater: swap failed and rollback failed: swap: %w; rollback: %v; original binary preserved at %s", err, rbErr, oldPath)
		}
		return fmt.Errorf("updater: swap failed before the new binary was in place (original restored; preserved at %s): %w", oldPath, err)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
