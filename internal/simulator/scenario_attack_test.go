package simulator

import "testing"

func TestDeeplyNestedHasRequestedDepth(t *testing.T) {
	t.Parallel()
	got := deeplyNested(3)
	if want := `{"x":{"x":{"x":1}}}`; got != want {
		t.Errorf("deeplyNested(3) = %q, want %q", got, want)
	}
}
