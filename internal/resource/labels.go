package resource

// MatchLabels reports whether all key-value pairs in selector are present and
// equal in labels. An empty selector matches everything.
func MatchLabels(selector, labels map[string]string) bool {
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}
