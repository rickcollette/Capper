package manager

import (
	"fmt"
	"os"
	"testing"

	"capper/internal/runtime"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 3 && os.Args[1] == "__run-limited" {
		if err := runtime.RunLimitedLauncher(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
