package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// warnOut receives warnings that must not fail the command; tests replace it.
var warnOut io.Writer = os.Stderr

// errGatesForbidden marks a caller without permission to list RolloutGates.
var errGatesForbidden = errors.New("not allowed to list RolloutGates")

// errLegacyGatesStale is returned, after the work is done, when a legacy
// approval gate still disagrees with the intended state and the caller has no
// right to edit it.
var errLegacyGatesStale = errors.New("legacy approval gates still disagree")

const (
	rolloutGateKind = "rolloutgates.kuberik.com"
	// gateLabelPrefix marks gates a controller generated; prettyNameLabel is
	// the one label under that prefix that does not.
	gateLabelPrefix = "gate.kuberik.com/"
	prettyNameLabel = "gate.kuberik.com/pretty-name"
	// legacyGateRetries bounds the conflict-retry loop of a gate update.
	legacyGateRetries = 5
)

type gateObj struct {
	Metadata struct {
		Name            string            `json:"name"`
		Labels          map[string]string `json:"labels"`
		OwnerReferences []json.RawMessage `json:"ownerReferences"`
	} `json:"metadata"`
	Spec struct {
		RolloutRef *struct {
			Name string `json:"name"`
		} `json:"rolloutRef"`
		AllowedVersions *[]string `json:"allowedVersions"`
	} `json:"spec"`
}

// isLegacyApprovalGate reports the ungraduated gate shape the approve action
// used to edit: ownerless, carrying no gate.kuberik.com marker label, with
// spec.allowedVersions set (non-nil, possibly empty). It matches the rule the
// dashboard applies, so both write the same gates.
func isLegacyApprovalGate(g *gateObj) bool {
	if g == nil || len(g.Metadata.OwnerReferences) != 0 || g.Spec.AllowedVersions == nil {
		return false
	}
	for k := range g.Metadata.Labels {
		if strings.HasPrefix(k, gateLabelPrefix) && k != prettyNameLabel {
			return false
		}
	}
	return true
}

// crdMissing reports a kubectl error for a resource type the cluster does not serve.
func crdMissing(err error) bool {
	s := err.Error()
	return strings.Contains(s, "doesn't have a resource type") || strings.Contains(s, "no matches for kind")
}

// setLegacyGateVersion adds (add) or removes the version in the allowedVersions
// of every legacy approval gate of the rollout, the migration dual-write the
// dashboard's Approve and Unapprove perform. Each gate is written with a
// resourceVersion-checked replace retried on conflict, so a concurrent edit is
// merged, never overwritten. An empty result stays [] (null would mean "no
// opinion" and open the gate). only, when non-nil, restricts the work to those
// gate names. It returns the names of the gates actually changed. A failed
// update rolls back the gates already changed in the call.
//
// A cluster without the RolloutGate CRD has no legacy gates: that is silent.
// A caller without permission to list gates gets errGatesForbidden; callers
// fall back to checkLegacyGatesFromStatus.
func setLegacyGateVersion(ns, rollout, version string, add bool, only map[string]bool) ([]string, error) {
	out, err := kubectlRun("", "-n", ns, "get", rolloutGateKind, "-o", "json")
	if err != nil {
		if crdMissing(err) {
			return nil, nil
		}
		if strings.Contains(err.Error(), "Forbidden") {
			return nil, errGatesForbidden
		}
		return nil, fmt.Errorf("list RolloutGates in %s: %w", ns, err)
	}
	var list struct {
		Items []gateObj `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		return nil, fmt.Errorf("list RolloutGates in %s: unreadable response: %w", ns, err)
	}
	var changed []string
	for i := range list.Items {
		g := &list.Items[i]
		if g.Spec.RolloutRef == nil || g.Spec.RolloutRef.Name != rollout || !isLegacyApprovalGate(g) {
			continue
		}
		if only != nil && !only[g.Metadata.Name] {
			continue
		}
		wrote, err := updateLegacyGate(ns, g.Metadata.Name, version, add)
		if err != nil {
			undoLegacyGates(ns, rollout, version, add, changed)
			return nil, err
		}
		if wrote {
			changed = append(changed, g.Metadata.Name)
		}
	}
	return changed, nil
}

// updateLegacyGate re-reads one gate and replaces it with the version added or
// removed, retrying when the replace loses a resourceVersion race.
func updateLegacyGate(ns, name, version string, add bool) (bool, error) {
	var lastErr error
	for attempt := 0; attempt < legacyGateRetries; attempt++ {
		out, err := kubectlRun("", "-n", ns, "get", rolloutGateKind, name, "-o", "json")
		if err != nil {
			if strings.Contains(err.Error(), "NotFound") {
				return false, nil
			}
			return false, fmt.Errorf("read RolloutGate %s/%s: %w", ns, name, err)
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(out), &raw); err != nil {
			return false, fmt.Errorf("read RolloutGate %s/%s: %w", ns, name, err)
		}
		var g gateObj
		if err := json.Unmarshal([]byte(out), &g); err != nil {
			return false, fmt.Errorf("read RolloutGate %s/%s: %w", ns, name, err)
		}
		if !isLegacyApprovalGate(&g) {
			return false, nil
		}
		next := make([]string, 0, len(*g.Spec.AllowedVersions)+1)
		has := false
		for _, v := range *g.Spec.AllowedVersions {
			if v == version {
				has = true
				if !add {
					continue
				}
			}
			next = append(next, v)
		}
		if add == has {
			return false, nil // already in the wanted state
		}
		if add {
			next = append(next, version)
		}
		spec, _ := raw["spec"].(map[string]any)
		spec["allowedVersions"] = next
		body, err := json.Marshal(raw)
		if err != nil {
			return false, err
		}
		if _, err := kubectlRun(string(body), "-n", ns, "replace", "-f", "-"); err != nil {
			if strings.Contains(err.Error(), "Conflict") || strings.Contains(err.Error(), "the object has been modified") {
				lastErr = err
				continue
			}
			return false, fmt.Errorf("update legacy approval gate %s/%s: %w", ns, name, err)
		}
		return true, nil
	}
	return false, fmt.Errorf("update legacy approval gate %s/%s: %w", ns, name, lastErr)
}

// undoLegacyGates best-effort reverts gate changes made by an operation whose
// RolloutApproval write then failed.
func undoLegacyGates(ns, rollout, version string, add bool, names []string) {
	if len(names) == 0 {
		return
	}
	only := map[string]bool{}
	for _, n := range names {
		only[n] = true
	}
	if _, err := setLegacyGateVersion(ns, rollout, version, !add, only); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "warning: could not undo the legacy approval gate change for %s/%s %q: %v\n", ns, rollout, version, err)
	}
}

// checkLegacyGatesFromStatus is the fallback for a caller who cannot list
// RolloutGates. The Rollout's status.gates (readable with get rollouts) carries
// each gate's name and allowedVersions. It returns the gates whose
// allowedVersions disagree with the wanted state: for add, a non-nil list
// lacking the version; otherwise a list still containing it. If the status
// cannot be read it warns once and returns nil.
func checkLegacyGatesFromStatus(ns, rollout, version string, add bool) []string {
	out, err := kubectlRun("", "-n", ns, "get", rolloutKind, rollout, "-o", "json")
	var ro struct {
		Status struct {
			Gates []struct {
				Name            string    `json:"name"`
				AllowedVersions *[]string `json:"allowedVersions"`
			} `json:"gates"`
		} `json:"status"`
	}
	if err == nil {
		err = json.Unmarshal([]byte(out), &ro)
	}
	if err != nil {
		_, _ = fmt.Fprintf(warnOut, "warning: could not check legacy approval gates of %s/%s (not allowed to list RolloutGates, status unreadable: %v); someone with RolloutGate update rights should check that the gates allow exactly the versions you intend\n", ns, rollout, err)
		return nil
	}
	var stale []string
	for _, g := range ro.Status.Gates {
		if g.AllowedVersions == nil {
			continue
		}
		has := false
		for _, v := range *g.AllowedVersions {
			has = has || v == version
		}
		if has != add {
			stale = append(stale, g.Name)
		}
	}
	return stale
}

// warnStaleLegacyGates tells the caller which gates could not be updated.
func warnStaleLegacyGates(ns, rollout, version string, add bool, gates []string) {
	if len(gates) == 0 {
		return
	}
	state := "still allows"
	want := "remove"
	if add {
		state = "does not yet allow"
		want = "add"
	}
	_, _ = fmt.Fprintf(warnOut, "warning: you are not allowed to list RolloutGates, so legacy approval gate(s) %s of %s/%s were not updated and %s %q. Someone with RolloutGate update rights must edit each gate's spec.allowedVersions and %s that version.\n", strings.Join(gates, ", "), ns, rollout, state, version, want)
}
