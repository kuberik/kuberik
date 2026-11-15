package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
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
	// rolloutStatus is the status object of the Rollout; rolloutErr fails reading it.
	rolloutStatus map[string]any
	rolloutErr    error
	rolloutGets   int
	rolloutOK     int // reads that succeed before rolloutErr applies
	listApprovals error
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
		f.rolloutGets++
		// The first read fetches the uid; later ones are the status fallback.
		if f.rolloutErr != nil && f.rolloutGets > f.rolloutOK {
			return "", f.rolloutErr
		}
		b, _ := json.Marshal(map[string]any{"metadata": map[string]any{"uid": "u"}, "status": f.rolloutStatus})
		return string(b), nil
	case args[2] == "get" && args[3] == rolloutGateKind && len(args) == 6:
		if f.listGates != nil {
			return "", f.listGates
		}
		names := []string{}
		for n := range f.gates {
			names = append(names, n)
		}
		sort.Strings(names)
		items := []any{}
		for _, n := range names {
			items = append(items, f.gates[n])
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
		if f.listApprovals != nil {
			return "", f.listApprovals
		}
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

func captureWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := warnOut
	warnOut = &buf
	t.Cleanup(func() { warnOut = old })
	return &buf
}

func gateStatus(entries ...any) map[string]any {
	return map[string]any{"gates": entries}
}

func statusGate(name string, versions []string) map[string]any {
	g := map[string]any{"name": name}
	if versions != nil {
		g["allowedVersions"] = versions
	}
	return g
}

const forbiddenGates = `Error from server (Forbidden): rolloutgates.kuberik.com is forbidden`

func TestApproveForbiddenGatesWarnsFromRolloutStatus(t *testing.T) {
	f := newFakeCluster()
	f.listGates = errors.New(forbiddenGates)
	f.rolloutStatus = gateStatus(statusGate("manual", []string{}), statusGate("done", []string{"v1.42.0"}), statusGate("kill", nil))
	useCluster(t, f)
	warn := captureWarnings(t)
	if _, err := approveVersion("prod", "checkout", "v1.42.0"); err != nil {
		t.Fatal(err)
	}
	got := warn.String()
	if !strings.Contains(got, "manual") || strings.Contains(got, "done") || strings.Contains(got, "kill") || !strings.Contains(got, "RolloutGate update rights") {
		t.Fatalf("warning = %q", got)
	}
}

func TestApproveForbiddenGatesAllInSyncIsSilent(t *testing.T) {
	f := newFakeCluster()
	f.listGates = errors.New(forbiddenGates)
	f.rolloutStatus = gateStatus(statusGate("done", []string{"v1.42.0"}))
	useCluster(t, f)
	warn := captureWarnings(t)
	if _, err := approveVersion("prod", "checkout", "v1.42.0"); err != nil {
		t.Fatal(err)
	}
	if warn.Len() != 0 {
		t.Fatalf("unexpected warning %q", warn.String())
	}
}

func TestRevokeForbiddenGatesWarnsAndFailsAfterDelete(t *testing.T) {
	f := newFakeCluster()
	f.listGates = errors.New(forbiddenGates)
	f.rolloutStatus = gateStatus(statusGate("manual", []string{"v1.42.0"}), statusGate("other", []string{"v1.1.0"}))
	f.approvals["checkout-v1-42-0"] = buildApproval("prod", "checkout", "u", "v1.42.0", "")
	useCluster(t, f)
	warn := captureWarnings(t)
	deleted, _, err := revokeVersion("prod", "checkout", "v1.42.0")
	if !errors.Is(err, errLegacyGatesStale) || !deleted {
		t.Fatalf("deleted=%v err=%v", deleted, err)
	}
	if _, ok := f.approvals["checkout-v1-42-0"]; ok {
		t.Fatal("approval should have been deleted")
	}
	got := warn.String()
	if !strings.Contains(got, "manual") || strings.Contains(got, "other") || !strings.Contains(got, "RolloutGate update rights") {
		t.Fatalf("warning = %q", got)
	}
}

func TestRevokeForbiddenGatesSkipsCheckWhileAnotherApprovalRemains(t *testing.T) {
	f := newFakeCluster()
	f.listGates = errors.New(forbiddenGates)
	f.rolloutStatus = gateStatus(statusGate("manual", []string{"v1.42.0"}))
	f.approvals["checkout-v1-42-0"] = buildApproval("prod", "checkout", "u", "v1.42.0", "")
	other := buildApproval("prod", "checkout", "u", "v1.42.0", "")
	other["metadata"].(map[string]any)["name"] = "release-1-42"
	f.approvals["release-1-42"] = other
	useCluster(t, f)
	warn := captureWarnings(t)
	if _, remaining, err := revokeVersion("prod", "checkout", "v1.42.0"); err != nil || len(remaining) != 1 {
		t.Fatalf("remaining=%v err=%v", remaining, err)
	}
	if warn.Len() != 0 {
		t.Fatalf("unexpected warning %q", warn.String())
	}
}

func TestForbiddenGatesAndUnreadableStatusWarnsUnconditionally(t *testing.T) {
	f := newFakeCluster()
	f.listGates = errors.New(forbiddenGates)
	f.rolloutErr = errors.New("Error from server (Forbidden): rollouts is forbidden")
	f.approvals["checkout-v1-42-0"] = buildApproval("prod", "checkout", "u", "v1.42.0", "")
	useCluster(t, f)
	warn := captureWarnings(t)
	if _, _, err := revokeVersion("prod", "checkout", "v1.42.0"); err != nil {
		t.Fatalf("unverifiable gates must not fail revoke: %v", err)
	}
	if !strings.Contains(warn.String(), "could not check legacy approval gates") {
		t.Fatalf("warning = %q", warn.String())
	}
	warn.Reset()
	f.rolloutGets, f.rolloutOK = 0, 1
	if _, err := approveVersion("prod", "checkout", "v1.42.0"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warn.String(), "could not check legacy approval gates") {
		t.Fatalf("warning = %q", warn.String())
	}
}

func TestMissingGateCRDIsSilent(t *testing.T) {
	f := newFakeCluster()
	f.listGates = errors.New(`error: the server doesn't have a resource type "rolloutgates"`)
	f.rolloutStatus = gateStatus(statusGate("manual", []string{}))
	useCluster(t, f)
	warn := captureWarnings(t)
	if _, err := approveVersion("prod", "checkout", "v1.42.0"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := revokeVersion("prod", "checkout", "v1.42.0"); err != nil {
		t.Fatal(err)
	}
	if warn.Len() != 0 {
		t.Fatalf("unexpected warning %q", warn.String())
	}
}

func TestMultiGateFailureRollsBackEarlierGates(t *testing.T) {
	f := newFakeCluster()
	f.addGate("a-gate", "checkout", []string{"v1.40.0"}, nil)
	f.addGate("b-gate", "checkout", []string{}, nil)
	f.replaceErr = func(name string, _ int) error {
		if name == "b-gate" {
			return errors.New(`Error from server (Forbidden): rolloutgates is forbidden`)
		}
		return nil
	}
	useCluster(t, f)
	if _, err := approveVersion("prod", "checkout", "v1.42.0"); err == nil {
		t.Fatal("want error")
	}
	if got := strings.Join(f.versions(t, "a-gate"), ","); got != "v1.40.0" {
		t.Fatalf("a-gate not restored: %q", got)
	}
	if f.replaces["a-gate"] != 2 {
		t.Fatalf("a-gate replaces = %d, want write then restore", f.replaces["a-gate"])
	}
	if len(f.approvals) != 0 {
		t.Fatal("approval created")
	}
}

func TestRevokeFailedApprovalListIsAnError(t *testing.T) {
	f := newFakeCluster()
	f.addGate("manual", "checkout", []string{"v1.42.0"}, nil)
	f.approvals["checkout-v1-42-0"] = buildApproval("prod", "checkout", "u", "v1.42.0", "")
	f.listApprovals = errors.New(`Error from server (Forbidden): rolloutapprovals is forbidden`)
	useCluster(t, f)
	if _, _, err := revokeVersion("prod", "checkout", "v1.42.0"); err == nil || !strings.Contains(err.Error(), "list RolloutApprovals") {
		t.Fatalf("want list error, got %v", err)
	}
	if got := strings.Join(f.versions(t, "manual"), ","); got != "v1.42.0" {
		t.Fatalf("gate changed despite failed list: %q", got)
	}
	if _, ok := f.approvals["checkout-v1-42-0"]; !ok {
		t.Fatal("approval deleted despite failed list")
	}
}
