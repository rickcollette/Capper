package manager

import "testing"

func TestRandomNameRetriesOnCollision(t *testing.T) {
	calls := 0
	name, err := randomName("hello.cap", func(candidate string) (bool, error) {
		calls++
		return calls == 1, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if name == "" {
		t.Fatal("expected name")
	}
	if calls < 2 {
		t.Fatalf("expected retry, got %d calls", calls)
	}
}
