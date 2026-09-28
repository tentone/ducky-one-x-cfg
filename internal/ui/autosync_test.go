package ui

import "testing"

func TestWithoutAutoSyncRestoresSuppression(t *testing.T) {
	u := &UI{}
	u.withoutAutoSync(func() {
		if u.autoSyncState.suppressed != 1 {
			t.Fatalf("suppression level = %d, want 1", u.autoSyncState.suppressed)
		}
		u.withoutAutoSync(func() {
			if u.autoSyncState.suppressed != 2 {
				t.Fatalf("nested suppression level = %d, want 2", u.autoSyncState.suppressed)
			}
		})
	})
	if u.autoSyncState.suppressed != 0 {
		t.Fatalf("suppression level after callback = %d, want 0", u.autoSyncState.suppressed)
	}
}
