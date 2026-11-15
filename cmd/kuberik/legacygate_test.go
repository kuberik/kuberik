package main

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
)

// fakeCluster is a tiny stateful stand-in for the kubectl calls approve and
// revoke make: a Rollout, RolloutGates and RolloutApprovals in namespace "prod".
type fakeCluster struct {
	mu        sync.Mutex
	gates     map[string]map[string]any // name -> object
	approvals map[string]map[string]any
	rv        map[string]int
	// replaceErr, when set, is returned by a replace of a gate (after the call is recorded).
	replaceErr func(name string, n int) error
	replaces   map[string]int
	whoami     string
	listGates  error
}

func newFakeCluster() *fakeCluster {
	return &fakeCluster{gates: map[string]map[string]any{}, approvals: map[string]map[string]any{}, rv: map[string]int{}, replaces: map[string]int{}}
}

func (f *fakeCluster) addGate(name, rollout string, versions []string, mutate func(meta map[string]any)) {
	meta := map[string]any{"name": name, "namespace": "prod", "resourceVersion": "1"}
	spec := map[string]any{"rolloutRef": map[string]any{"name": rollout}}
	if versions != nil {
		spec["allowedVersions"] = versions
	}
	if mutate != nil {
		mutate(meta)
	}
	f.gates[name] = map[string]any{"apiVersion": kuberikAPIVersion, "kind": "RolloutGate", "metadata": meta, "spec": spec}
}

func (f *fakeCluster) versions(t *testing.T, name string) []string {
	t.Helper()
	spec := f.gates[name]["spec"].(map[string]any)
	switch v := spec["allowedVersions"].(type) {
	case []string:
		return v
	case []any:
		out := []string{}
		for _, x := range v {
			out = append(out, x.(string))
		}
		return out
	case nil:
		t.Fatalf("gate %s allowedVersions is null (would mean no opinion)", name)
	}
	t.Fatalf("gate %s: unexpected allowedVersions", name)
	return nil
}

func (f *fakeCluster) run(stdin string, args []string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case args[0] == "auth":
		return `{"status":{"userInfo":{"username":"` + f.whoami + `"}}}`, nil
	case args[2] == "get" && args[3] == rolloutKind:
		return `{"metadata":{"uid":"u"}}`, nil
	case args[2] == "get" && args[3] == rolloutGateKind && len(args) == 6:
		if f.listGates != nil {
			return "", f.listGates
		}
		items := []any{}
		for _, g := range f.gates {
			items = append(items, g)
		}
		b, _ := json.Marshal(map[string]any{"items": items})
		return string(b), nil
	case args[2] == "get" && args[3] == rolloutGateKind:
		g, ok := f.gates[args[4]]
		if !ok {
			return "", errors.New("Error from server (NotFound)")
		}
		b, _ := json.Marshal(g)
		return string(b), nil
	case args[2] == "replace":
		var obj map[string]any
		if err := json.Unmarshal([]byte(stdin), &obj); err != nil {
			return "", err
		}
		name := obj["metadata"].(map[string]any)["name"].(string)
		f.replaces[name]++
		if f.replaceErr != nil {
			if err := f.replaceErr(name, f.replaces[name]); err != nil {
				return "", err
			}
		}
		f.gates[name] = obj
		return "", nil
	case args[0] == "create":
		var obj map[string]any
		_ = json.Unmarshal([]byte(stdin), &obj)
		name := obj["metadata"].(map[string]any)["name"].(string)
		if _, ok := f.approvals[name]; ok {
			return "", errors.New("Error from server (AlreadyExists)")
		}
		f.approvals[name] = obj
		return "", nil
	case args[2] == "get" && args[3] == rolloutApprovalKind && len(args) == 7:
		a, ok := f.approvals[args[4]]
		if !ok {
			return "", errors.New("Error from server (NotFound)")
		}
		b, _ := json.Marshal(a)
		return string(b), nil
	case args[2] == "get" && args[3] == rolloutApprovalKind:
		items := []any{}
		for _, a := range f.approvals {
			items = append(items, a)
		}
		b, _ := json.Marshal(map[string]any{"items": items})
		return string(b), nil
	case args[2] == "delete":
		delete(f.approvals, args[4])
		return "", nil
	}
	return "", errors.New("unexpected kubectl " + strings.Join(args, " "))
}

func useCluster(t *testing.T, f *fakeCluster) {
	t.Helper()
	fakeKubectl(t, func(stdin string, args []string) (string, error) { return f.run(stdin, args) })
}

func TestApproveLegacyGateIsDualWrittenAndOtherGatesLeftAlone(t *testing.T) {
	f := newFakeCluster()
	f.addGate("manual", "checkout", []string{"v1.40.0"}, nil)
	f.addGate("policy-gate", "checkout", []string{}, func(m map[string]any) {
		m["labels"] = map[string]any{"gate.kuberik.com/approval-policy-name": "p"}
		m["ownerReferences"] = []any{map[string]any{"kind": "ApprovalPolicy", "name": "p"}}
	})
	f.addGate("sched", "checkout", []string{}, func(m map[string]any) {
		m["ownerReferences"] = []any{map[string]any{"kind": "RolloutSchedule", "name": "s"}}
	})
	f.addGate("kill", "checkout", nil, nil)
	f.addGate("other-rollout", "billing", []string{}, nil)
	f.addGate("pretty", "checkout", []string{}, func(m map[string]any) {
		m["labels"] = map[string]any{"gate.kuberik.com/pretty-name": "Manual"}
	})
	useCluster(t, f)

	if _, err := approveVersion("prod", "checkout", "v1.42.0"); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.approvals["checkout-v1-42-0"]; !ok {
		t.Fatal("approval not created")
	}
	if got := strings.Join(f.versions(t, "manual"), ","); got != "v1.40.0,v1.42.0" {
		t.Fatalf("legacy gate allowedVersions = %s", got)
	}
	if got := strings.Join(f.versions(t, "pretty"), ","); got != "v1.42.0" {
		t.Fatalf("pretty-name label must not exempt a legacy gate, got %s", got)
	}
	for _, n := range []string{"policy-gate", "sched", "other-rollout"} {
		if got := f.versions(t, n); len(got) != 0 {
			t.Fatalf("%s was written: %v", n, got)
		}
	}
	if f.gates["kill"]["spec"].(map[string]any)["allowedVersions"] != nil {
		t.Fatal("kill-switch gate (no allowedVersions) was written")
	}
	// Idempotent: no duplicate and no extra write on a second approve.
	n := f.replaces["manual"]
	if _, err := approveVersion("prod", "checkout", "v1.42.0"); !errors.Is(err, errAlreadyApproved) {
		t.Fatalf("second approve: %v", err)
	}
	if got := strings.Join(f.versions(t, "manual"), ","); got != "v1.40.0,v1.42.0" || f.replaces["manual"] != n {
		t.Fatalf("after second approve = %s (replaces %d -> %d)", got, n, f.replaces["manual"])
	}
	// Revoke removes it again and keeps [] rather than null.
	if _, _, err := revokeVersion("prod", "checkout", "v1.42.0"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(f.versions(t, "manual"), ","); got != "v1.40.0" {
		t.Fatalf("after revoke = %s", got)
	}
	f.approvals["checkout-v1-40-0"] = buildApproval("prod", "checkout", "u", "v1.40.0", "")
	if _, _, err := revokeVersion("prod", "checkout", "v1.40.0"); err != nil {
		t.Fatal(err)
	}
	if got := f.versions(t, "manual"); len(got) != 0 {
		t.Fatalf("after last revoke = %v", got)
	}
}

func TestRevokeLegacyGateKeepsVersionWhileAnotherApprovalExists(t *testing.T) {
	f := newFakeCluster()
	f.addGate("manual", "checkout", []string{}, nil)
	useCluster(t, f)
	committed := buildApproval("prod", "checkout", "u", "v1.42.0", "")
	committed["metadata"].(map[string]any)["name"] = "release-1-42"
	f.approvals["release-1-42"] = committed
	if _, err := approveVersion("prod", "checkout", "v1.42.0"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := revokeVersion("prod", "checkout", "v1.42.0"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(f.versions(t, "manual"), ","); got != "v1.42.0" {
		t.Fatalf("legacy gate lost a version another approval still grants: %q", got)
	}
}

func TestApproveLegacyGateRetriesOnConflict(t *testing.T) {
	f := newFakeCluster()
	f.addGate("manual", "checkout", []string{}, nil)
	f.replaceErr = func(_ string, n int) error {
		if n <= 2 {
			return errors.New(`Error from server (Conflict): Operation cannot be fulfilled: the object has been modified`)
		}
		return nil
	}
	useCluster(t, f)
	if _, err := approveVersion("prod", "checkout", "v1.42.0"); err != nil {
		t.Fatal(err)
	}
	if f.replaces["manual"] != 3 || strings.Join(f.versions(t, "manual"), ",") != "v1.42.0" {
		t.Fatalf("replaces=%d versions=%v", f.replaces["manual"], f.versions(t, "manual"))
	}
}

func TestApproveWithoutGateRightsAndNoLegacyGateSucceeds(t *testing.T) {
	f := newFakeCluster()
	f.listGates = errors.New(`Error from server (Forbidden): rolloutgates.kuberik.com is forbidden`)
	useCluster(t, f)
	if _, err := approveVersion("prod", "checkout", "v1.42.0"); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.approvals["checkout-v1-42-0"]; !ok {
		t.Fatal("RolloutApproval not created")
	}
	if deleted, _, err := revokeVersion("prod", "checkout", "v1.42.0"); err != nil || !deleted {
		t.Fatalf("deleted=%v err=%v", deleted, err)
	}
}

func TestApproveLegacyGateForbiddenLeavesNothingCreated(t *testing.T) {
	f := newFakeCluster()
	f.addGate("manual", "checkout", []string{}, nil)
	f.replaceErr = func(string, int) error { return errors.New(`Error from server (Forbidden): rolloutgates is forbidden`) }
	useCluster(t, f)
	if _, err := approveVersion("prod", "checkout", "v1.42.0"); err == nil || !strings.Contains(err.Error(), "Forbidden") {
		t.Fatalf("want forbidden error, got %v", err)
	}
	if len(f.approvals) != 0 {
		t.Fatal("RolloutApproval was created despite the refused gate update")
	}
}

func TestRevokeLegacyGateForbiddenLeavesApprovalInPlace(t *testing.T) {
	f := newFakeCluster()
	f.addGate("manual", "checkout", []string{"v1.42.0"}, nil)
	f.approvals["checkout-v1-42-0"] = buildApproval("prod", "checkout", "u", "v1.42.0", "")
	f.replaceErr = func(string, int) error { return errors.New(`Error from server (Forbidden): rolloutgates is forbidden`) }
	useCluster(t, f)
	if _, _, err := revokeVersion("prod", "checkout", "v1.42.0"); err == nil {
		t.Fatal("want error")
	}
	if _, ok := f.approvals["checkout-v1-42-0"]; !ok {
		t.Fatal("RolloutApproval was deleted despite the refused gate update")
	}
}

func TestApproveCollisionUndoesGateWrite(t *testing.T) {
	f := newFakeCluster()
	f.addGate("manual", "checkout", []string{}, nil)
	f.approvals["checkout-v1-42-0"] = buildApproval("prod", "checkout", "u", "v1.42.0+x", "")
	useCluster(t, f)
	if _, err := approveVersion("prod", "checkout", "v1.42.0"); !errors.Is(err, errApprovalCollision) {
		t.Fatalf("want collision, got %v", err)
	}
	if got := f.versions(t, "manual"); len(got) != 0 {
		t.Fatalf("gate write not undone: %v", got)
	}
}
