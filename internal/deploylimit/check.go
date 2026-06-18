package deploylimit

import "fmt"

// CheckCount returns an error when current deployments meet or exceed the host cap.
func CheckCount(current int64) error {
	max := MaxDeployments()
	if current >= max {
		return fmt.Errorf("host deployment limit reached (%d/%d capsules)", current, max)
	}
	return nil
}
