package deploylimit

import "fmt"

// CheckCount returns an error when current deployments meet or exceed the host cap.
func CheckCount(current int64) error {
	return CheckCountWithMax(current, MaxDeployments())
}

// CheckCountWithMax is CheckCount against an explicit cap (e.g. an admin-set
// override resolved from persisted config).
func CheckCountWithMax(current, max int64) error {
	if current >= max {
		return fmt.Errorf("host deployment limit reached (%d/%d capsules)", current, max)
	}
	return nil
}

// ResolveMax returns the effective host deployment cap: a positive admin
// override takes precedence; otherwise the env/RAM-derived default applies.
func ResolveMax(override int64) int64 {
	if override > 0 {
		return override
	}
	return MaxDeployments()
}
