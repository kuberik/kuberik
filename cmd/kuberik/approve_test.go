package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// Cases that pin the naming rule; it is shared byte for byte with the dashboard
// and rollout-controller.
func TestApprovalName(t *testing.T) {
	long := "v" + strings.Repeat("a", 300)
	sum := sha256.Sum256([]byte(long))
	cases := []struct{ rollout, version, want string }{
		{"checkout", "v1.42.0", "checkout-v1-42-0"},
		{"checkout", "V1.42.0+Build_7", "checkout-v1-42-0-build-7"},
		{"checkout", "1.0.0-", "checkout-1-0-0"},
		{"checkout", "v1-é", "checkout-v1"},
		{"checkout", "v1éx", "checkout-v1--x"},
		{"checkout", long, "checkout-v" + strings.Repeat("a", 44) + "-" + hex.EncodeToString(sum[:])[:8]},
	}
	for _, tc := range cases {
		got := approvalName(tc.rollout, tc.version)
		if got != tc.want {
			t.Errorf("approvalName(%q, %.20q) = %q, want %q", tc.rollout, tc.version, got, tc.want)
		}
		if len(got) > 63 {
			t.Errorf("name %q longer than 63", got)
		}
	}
	if got := approvalName("checkout", long); !strings.HasSuffix(got, "-42c4c913") {
		t.Errorf("long name %q: want the shell's sha256 prefix 42c4c913", got)
	}
	// n[:54] ends in the "-" that "." sanitises to; it is stripped before the hash.
	v := strings.Repeat("a", 44) + "." + strings.Repeat("b", 30)
	s := sha256.Sum256([]byte(v))
	want := "checkout-" + strings.Repeat("a", 44) + "-" + hex.EncodeToString(s[:])[:8]
	if got := approvalName("checkout", v); got != want {
		t.Errorf("truncation: got %q, want %q", got, want)
	}
}

func TestApproverIdentity(t *testing.T) {
	for user, want := range map[string]string{
		"oidc:luka":                   "oidc:luka",
		"":                            "",
		"system:serviceaccount:ns:sa": "",
	} {
		if got := approverIdentity(user); got != want {
			t.Errorf("approverIdentity(%q) = %q, want %q", user, got, want)
		}
	}
}

func TestBuildApproval(t *testing.T) {
	b, _ := json.Marshal(buildApproval("prod", "checkout", "uid-1", "v1.2.3", "luka"))
	var a struct {
		Metadata struct {
			Name, Namespace string
			Annotations     map[string]string
			OwnerReferences []struct {
				APIVersion, Kind, Name, UID string
			}
		}
		Spec struct {
			RolloutRef struct{ Name string }
			Version    string
		}
	}
	if err := json.Unmarshal(b, &a); err != nil {
		t.Fatal(err)
	}
	if a.Metadata.Name != "checkout-v1-2-3" || a.Metadata.Namespace != "prod" || a.Spec.Version != "v1.2.3" || a.Spec.RolloutRef.Name != "checkout" {
		t.Errorf("unexpected object: %s", b)
	}
	o := a.Metadata.OwnerReferences
	if len(o) != 1 || o[0].APIVersion != "kuberik.com/v1alpha1" || o[0].Kind != "Rollout" || o[0].Name != "checkout" || o[0].UID != "uid-1" {
		t.Errorf("ownerReferences: %s", b)
	}
	if a.Metadata.Annotations["approval.kuberik.com/by"] != "luka" {
		t.Errorf("by annotation missing: %s", b)
	}
	if strings.Contains(string(mustJSON(buildApproval("prod", "checkout", "u", "v1", ""))), "annotations") {
		t.Error("empty by must omit annotations")
	}
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

// fakeKubectl swaps kubectlRun for a scripted fake and records the calls.
func fakeKubectl(t *testing.T, fn func(stdin string, args []string) (string, error)) *[]string {
	t.Helper()
	var calls []string
	old := kubectlRun
	kubectlRun = func(stdin string, args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		return fn(stdin, args)
	}
	t.Cleanup(func() { kubectlRun = old })
	return &calls
}

func TestApproveVersionIdempotent(t *testing.T) {
	existing := string(mustJSON(buildApproval("prod", "checkout", "u", "v1.2.3", "")))
	var created string
	fakeKubectl(t, func(stdin string, args []string) (string, error) {
		switch {
		case args[0] == "auth":
			return `{"status":{"userInfo":{"username":"luka"}}}`, nil
		case args[2] == "get" && args[3] == rolloutKind:
			return `{"metadata":{"uid":"u"}}`, nil
		case args[2] == "get" && args[3] == rolloutGateKind:
			return `{"items":[]}`, nil
		case args[0] == "create":
			created = stdin
			return "", errors.New("exit 1: Error from server (AlreadyExists): rolloutapprovals already exists")
		case args[2] == "get" && args[3] == rolloutApprovalKind:
			return existing, nil
		}
		t.Fatalf("unexpected kubectl %v", args)
		return "", nil
	})
	name, err := approveVersion("prod", "checkout", "v1.2.3")
	if !errors.Is(err, errAlreadyApproved) || name != "checkout-v1-2-3" {
		t.Fatalf("got %q, %v; want already approved", name, err)
	}
	if !strings.Contains(created, `"approval.kuberik.com/by":"luka"`) {
		t.Errorf("created object lacks the by annotation: %s", created)
	}
}

func TestApproveVersionCollision(t *testing.T) {
	other := string(mustJSON(buildApproval("prod", "checkout", "u", "v1.2.3+x", "")))
	fakeKubectl(t, func(stdin string, args []string) (string, error) {
		switch {
		case args[0] == "auth":
			return "", errors.New("no whoami")
		case args[2] == "get" && args[3] == rolloutKind:
			return `{"metadata":{"uid":"u"}}`, nil
		case args[2] == "get" && args[3] == rolloutGateKind:
			return `{"items":[]}`, nil
		case args[0] == "create":
			return "", errors.New("AlreadyExists")
		}
		return other, nil
	})
	if _, err := approveVersion("prod", "checkout", "v1.2.3"); !errors.Is(err, errApprovalCollision) {
		t.Fatalf("want collision error, got %v", err)
	}
}

func TestRevokeVersion(t *testing.T) {
	ours := string(mustJSON(buildApproval("prod", "checkout", "u", "v1", "")))
	t.Run("deletes ours", func(t *testing.T) {
		calls := fakeKubectl(t, func(_ string, args []string) (string, error) {
			if args[2] == "get" && len(args) == 7 {
				return ours, nil
			}
			return `{"items":[]}`, nil
		})
		deleted, _, err := revokeVersion("prod", "checkout", "v1")
		if err != nil || !deleted || !strings.Contains(strings.Join(*calls, "\n"), "delete rolloutapprovals.kuberik.com checkout-v1") {
			t.Fatalf("deleted=%v err=%v calls=%v", deleted, err, *calls)
		}
	})
	t.Run("NotFound is success", func(t *testing.T) {
		calls := fakeKubectl(t, func(_ string, args []string) (string, error) {
			if args[2] == "get" && len(args) == 7 {
				return "", errors.New(`Error from server (NotFound): not found`)
			}
			return `{"items":[]}`, nil
		})
		deleted, _, err := revokeVersion("prod", "checkout", "v1")
		if err != nil || deleted {
			t.Fatalf("deleted=%v err=%v", deleted, err)
		}
		for _, c := range *calls {
			if strings.HasPrefix(c, "-n prod delete") {
				t.Errorf("must not delete: %v", *calls)
			}
		}
	})
	t.Run("reports other approvals", func(t *testing.T) {
		gitOne := mustJSON(buildApproval("prod", "checkout", "u", "v1", ""))
		var list map[string]any
		_ = json.Unmarshal([]byte(`{"items":[]}`), &list)
		item := map[string]any{}
		_ = json.Unmarshal(gitOne, &item)
		item["metadata"].(map[string]any)["name"] = "from-git"
		fakeKubectl(t, func(_ string, args []string) (string, error) {
			if args[2] == "get" && len(args) == 7 {
				return ours, nil
			}
			return string(mustJSON(map[string]any{"items": []any{item}})), nil
		})
		_, rem, err := revokeVersion("prod", "checkout", "v1")
		if err != nil || len(rem) != 1 || rem[0] != "from-git" {
			t.Fatalf("remaining=%v err=%v", rem, err)
		}
	})
}
