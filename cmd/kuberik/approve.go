package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	rolloutKind          = "rollouts.kuberik.com"
	rolloutApprovalKind  = "rolloutapprovals.kuberik.com"
	kuberikAPIVersion    = "kuberik.com/v1alpha1"
	serviceAccountPrefix = "system:serviceaccount:"

	// annotationApprovalBy records who approved. The rollout-controller's
	// admission policy makes it equal to the requesting user where enforced.
	annotationApprovalBy = "approval.kuberik.com/by"
)

// errApprovalCollision means the deterministic name is taken by an approval of
// a different rollout or version (the name rule is lossy).
var errApprovalCollision = errors.New("a RolloutApproval with this name already exists for a different rollout or version")

// errAlreadyApproved is returned by approveVersion when an identical approval exists.
var errAlreadyApproved = errors.New("already approved")

// approvalName is the deterministic RolloutApproval name for a rollout and a
// version tag. It MUST stay byte for byte identical with ApprovalName in
// rollout-dashboard (pkg/kubernetes/approvals.go) and hack/migrate-approval-gates.sh
// in rollout-controller:
//
//  1. s = version sanitised byte by byte, ASCII only: A-Z lowercased, [a-z0-9-]
//     kept, every other byte (each byte of a multi-byte character) becomes "-".
//  2. n = rollout + "-" + s, trailing "-" stripped.
//  3. len(n) <= 63: n.
//  4. otherwise: first 54 chars of n with trailing "-" and "." stripped, then
//     "-" + first 8 hex chars of sha256(version) (raw bytes).
func approvalName(rollout, version string) string {
	var b strings.Builder
	for i := 0; i < len(version); i++ {
		ch := version[i]
		switch {
		case ch >= 'A' && ch <= 'Z':
			b.WriteByte(ch + ('a' - 'A'))
		case (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-':
			b.WriteByte(ch)
		default:
			b.WriteByte('-')
		}
	}
	n := strings.TrimRight(rollout+"-"+b.String(), "-")
	if len(n) <= 63 {
		return n
	}
	sum := sha256.Sum256([]byte(version))
	h := hex.EncodeToString(sum[:])[:8]
	return strings.TrimRight(n[:54], "-.") + "-" + h
}

// approverIdentity is the `by` value for an approval: the username when known
// and not a service account; "" omits the annotation.
func approverIdentity(username string) string {
	if username == "" || strings.HasPrefix(username, serviceAccountPrefix) {
		return ""
	}
	return username
}

// buildApproval returns the RolloutApproval the dashboard creates for
// (rollout, version): owned by the Rollout, spec.rolloutRef + spec.version.
func buildApproval(ns, rollout, uid, version, by string) map[string]any {
	meta := map[string]any{
		"name":      approvalName(rollout, version),
		"namespace": ns,
		"ownerReferences": []any{map[string]any{
			"apiVersion": kuberikAPIVersion,
			"kind":       "Rollout",
			"name":       rollout,
			"uid":        uid,
		}},
	}
	if by != "" {
		meta["annotations"] = map[string]any{annotationApprovalBy: by}
	}
	return map[string]any{
		"apiVersion": kuberikAPIVersion,
		"kind":       "RolloutApproval",
		"metadata":   meta,
		"spec": map[string]any{
			"rolloutRef": map[string]any{"name": rollout},
			"version":    version,
		},
	}
}

type approvalObj struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec struct {
		RolloutRef struct {
			Name string `json:"name"`
		} `json:"rolloutRef"`
		Version string `json:"version"`
	} `json:"spec"`
}

// currentUsername asks the API server who the caller is (kubectl auth whoami).
// Failure is not fatal: the approval is then created without the annotation,
// as the dashboard does when the identity lookup fails.
func currentUsername() string {
	out, err := kubectlRun("", "auth", "whoami", "-o", "json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not resolve your identity (%v); approving without %s\n", err, annotationApprovalBy)
		return ""
	}
	var r struct {
		Status struct {
			UserInfo struct {
				Username string `json:"username"`
			} `json:"userInfo"`
		} `json:"status"`
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		fmt.Fprintf(os.Stderr, "warning: unreadable identity response (%v); approving without %s\n", err, annotationApprovalBy)
		return ""
	}
	return r.Status.UserInfo.Username
}

// approveVersion creates the RolloutApproval and returns its name. An existing
// identical approval yields errAlreadyApproved (with the name); a name taken
// by a different rollout or version yields errApprovalCollision.
func approveVersion(ns, rollout, version string) (string, error) {
	out, err := kubectlRun("", "-n", ns, "get", rolloutKind, rollout, "-o", "json")
	if err != nil {
		return "", fmt.Errorf("get rollout %s/%s: %w", ns, rollout, err)
	}
	var ro struct {
		Metadata struct {
			UID string `json:"uid"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(out), &ro); err != nil || ro.Metadata.UID == "" {
		return "", fmt.Errorf("rollout %s/%s: unreadable response (no uid): %v", ns, rollout, err)
	}
	ap := buildApproval(ns, rollout, ro.Metadata.UID, version, approverIdentity(currentUsername()))
	body, err := json.Marshal(ap)
	if err != nil {
		return "", err
	}
	name := approvalName(rollout, version)
	// Legacy approval gates gain the version too (migration dual-write, as the
	// dashboard's Approve does). The gate write comes first so a refused gate
	// update leaves nothing half-written; it is undone if the create fails.
	changed, err := setLegacyGateVersion(ns, rollout, version, true, nil)
	forbidden := errors.Is(err, errGatesForbidden)
	if err != nil && !forbidden {
		return "", err
	}
	name, err = createApproval(ns, rollout, version, name, string(body))
	if err != nil && !errors.Is(err, errAlreadyApproved) {
		undoLegacyGates(ns, rollout, version, true, changed)
	}
	if forbidden && (err == nil || errors.Is(err, errAlreadyApproved)) {
		warnStaleLegacyGates(ns, rollout, version, true, checkLegacyGatesFromStatus(ns, rollout, version, true))
	}
	return name, err
}

// createApproval creates the RolloutApproval from body. An identical existing
// one yields errAlreadyApproved; a different one under the same name yields
// errApprovalCollision.
func createApproval(ns, rollout, version, name, body string) (string, error) {
	_, err := kubectlRun(body, "create", "-f", "-")
	if err == nil {
		return name, nil
	}
	if !strings.Contains(err.Error(), "AlreadyExists") {
		return "", fmt.Errorf("create RolloutApproval %s/%s: %w", ns, name, err)
	}
	existing, gerr := getApproval(ns, name)
	if gerr != nil {
		return "", gerr
	}
	if existing.Spec.RolloutRef.Name != rollout || existing.Spec.Version != version {
		return "", fmt.Errorf("%w: %s/%s approves %q for %q", errApprovalCollision, ns, name, existing.Spec.Version, existing.Spec.RolloutRef.Name)
	}
	return name, errAlreadyApproved
}

func getApproval(ns, name string) (*approvalObj, error) {
	out, err := kubectlRun("", "-n", ns, "get", rolloutApprovalKind, name, "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("read RolloutApproval %s/%s: %w", ns, name, err)
	}
	a := &approvalObj{}
	if err := json.Unmarshal([]byte(out), a); err != nil {
		return nil, fmt.Errorf("read RolloutApproval %s/%s: %w", ns, name, err)
	}
	return a, nil
}

// revokeVersion deletes the deterministic-name RolloutApproval (NotFound is
// success). It never deletes an object that approves something else. It
// also returns the names of other approvals that keep the version approved.
func revokeVersion(ns, rollout, version string) (deleted bool, remaining []string, err error) {
	name := approvalName(rollout, version)
	ours := false
	out, gerr := kubectlRun("", "-n", ns, "get", rolloutApprovalKind, name, "-o", "json")
	switch {
	case gerr == nil:
		a := &approvalObj{}
		if err := json.Unmarshal([]byte(out), a); err != nil {
			return false, nil, fmt.Errorf("read RolloutApproval %s/%s: %w", ns, name, err)
		}
		ours = a.Spec.RolloutRef.Name == rollout && a.Spec.Version == version
	case strings.Contains(gerr.Error(), "NotFound"):
	default:
		return false, nil, fmt.Errorf("read RolloutApproval %s/%s: %w", ns, name, gerr)
	}
	lout, lerr := kubectlRun("", "-n", ns, "get", rolloutApprovalKind, "-o", "json")
	switch {
	case lerr == nil:
		var list struct {
			Items []approvalObj `json:"items"`
		}
		if err := json.Unmarshal([]byte(lout), &list); err != nil {
			return false, nil, fmt.Errorf("list RolloutApprovals in %s: unreadable response: %w", ns, err)
		}
		for _, a := range list.Items {
			if a.Spec.RolloutRef.Name == rollout && a.Spec.Version == version && (!ours || a.Metadata.Name != name) {
				remaining = append(remaining, a.Metadata.Name)
			}
		}
	case crdMissing(lerr):
	default:
		return false, nil, fmt.Errorf("list RolloutApprovals in %s: %w", ns, lerr)
	}
	// Legacy gates lose the version only when nothing else still approves it.
	// Gate first: a refused gate update leaves the approval in place.
	var changed []string
	forbidden := false
	if len(remaining) == 0 {
		changed, err = setLegacyGateVersion(ns, rollout, version, false, nil)
		forbidden = errors.Is(err, errGatesForbidden)
		if err != nil && !forbidden {
			return false, nil, err
		}
	}
	if ours {
		if _, derr := kubectlRun("", "-n", ns, "delete", rolloutApprovalKind, name, "--ignore-not-found"); derr != nil {
			undoLegacyGates(ns, rollout, version, false, changed)
			return false, remaining, fmt.Errorf("delete RolloutApproval %s/%s: %w", ns, name, derr)
		}
	}
	if forbidden {
		// The approval is gone, but gates the caller cannot list may still allow the version.
		if stale := checkLegacyGatesFromStatus(ns, rollout, version, false); len(stale) > 0 {
			warnStaleLegacyGates(ns, rollout, version, false, stale)
			return ours, remaining, fmt.Errorf("%w: %s", errLegacyGatesStale, describeStaleGates(stale))
		}
	}
	return ours, remaining, nil
}

var approveCmd = &cobra.Command{
	Use:   "approve ROLLOUT VERSION",
	Short: "Approve a release candidate of a Rollout",
	Long: `Create the RolloutApproval for ROLLOUT and VERSION, the same object the
dashboard's Approve button creates. The rollout-controller turns it into an
allowed version on the Rollout's approval gate.

The object is named deterministically from the rollout and version, is owned
by the Rollout, and records you (kubectl auth whoami) in the
approval.kuberik.com/by annotation unless you are a service account.
Approving the same version twice is a no-op. Undo it with "kuberik revoke".`,
	Example: `  kuberik approve my-app v1.2.3 -n production`,
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireKubectl(); err != nil {
			return err
		}
		name, err := approveVersion(namespace, args[0], args[1])
		switch {
		case err == nil:
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Approved %s for %s/%s\n", args[1], namespace, args[0])
		case errors.Is(err, errAlreadyApproved):
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s is already approved for %s/%s\n", args[1], namespace, args[0])
		default:
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "rolloutapproval.kuberik.com/%s\n", name)
		return nil
	},
}

var revokeCmd = &cobra.Command{
	Use:   "revoke ROLLOUT VERSION",
	Short: "Withdraw the approval of a release candidate",
	Long: `Delete the RolloutApproval that "kuberik approve" (or the dashboard) created
for ROLLOUT and VERSION. A missing approval is not an error. Approvals
committed to git under other names are left alone and reported: they keep
the version approved.`,
	Example: `  kuberik revoke my-app v1.2.3 -n production`,
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireKubectl(); err != nil {
			return err
		}
		deleted, remaining, err := revokeVersion(namespace, args[0], args[1])
		if err != nil && !errors.Is(err, errLegacyGatesStale) {
			return err
		}
		staleErr := err
		w := cmd.ErrOrStderr()
		if deleted {
			_, _ = fmt.Fprintf(w, "Revoked approval of %s for %s/%s\n", args[1], namespace, args[0])
		} else {
			_, _ = fmt.Fprintf(w, "No approval of %s created by kuberik for %s/%s; nothing to delete\n", args[1], namespace, args[0])
		}
		if len(remaining) > 0 {
			_, _ = fmt.Fprintf(w, "Still approved by other RolloutApprovals: %s\n", strings.Join(remaining, ", "))
		}
		return staleErr
	},
}

func init() {
	rootCmd.AddCommand(approveCmd, revokeCmd)
}
