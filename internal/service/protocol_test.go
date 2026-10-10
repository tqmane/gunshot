package service

import (
	"strings"
	"testing"
)

func TestRolePermissions(t *testing.T) {
	// The public protocol boundary, independent of each operation's payload validation.
	permissions := map[string]string{
		"conditions":          "daemon",
		"upload_summary":      "photos googlephotos settings",
		"job":                 "photos googlephotos settings",
		"ping":                "photos googlephotos settings",
		"list":                "photos googlephotos settings",
		"accounts":            "photos googlephotos settings",
		"options":             "photos googlephotos settings",
		"retry":               "photos googlephotos settings",
		"cancel":              "photos googlephotos settings",
		"clear_completed":     "photos googlephotos settings",
		"retry_failed":        "photos googlephotos settings",
		"begin":               "photos googlephotos",
		"append":              "photos googlephotos",
		"seal":                "photos googlephotos",
		"account_native":      "googlephotos",
		"native_bearer":       "googlephotos",
		"native_bearer_clear": "googlephotos",
		"configure":           "googlephotos settings",
		"account_add":         "googlephotos settings",
		"account_remove":      "googlephotos settings",
		"account_select":      "googlephotos settings",
		"":                    "",
		"unknown":             "",
	}
	for op, allowed := range permissions {
		for _, role := range []string{"daemon", "googlephotos", "photos", "settings", "", "unknown"} {
			want := role != "" && strings.Contains(" "+allowed+" ", " "+role+" ")
			if got := roleAllowed(role, op); got != want {
				t.Errorf("roleAllowed(%q, %q) = %v, want %v", role, op, got, want)
			}
		}
	}
}
