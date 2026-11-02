package evaluation

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/renan-lab/fraud-analysis-go/internal/groundtruth"
)

func TestReadTruthKeepsFirstAcceptedLinePerTransaction(t *testing.T) {
	t.Parallel()
	f, err := os.Open(filepath.Join("testdata", "truth.ndjson"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	got, err := ReadTruth(f)
	if err != nil {
		t.Fatalf("ReadTruth(fixture) error = %v", err)
	}
	if got.Lines != 6 || got.NotAccepted != 3 {
		t.Errorf("ReadTruth(fixture) lines=%d notAccepted=%d, want 6 and 3", got.Lines, got.NotAccepted)
	}
	if want := []string{"t1", "t2"}; !reflect.DeepEqual(got.Order, want) {
		t.Errorf("ReadTruth(fixture) order = %v, want %v", got.Order, want)
	}
	if exp := got.Expectations["t2"]; exp.Label != groundtruth.LabelFraud || exp.Scenario != "fraud/velocity" {
		t.Errorf("ReadTruth(fixture) t2 = %+v, want the first line (fraud/velocity)", exp)
	}
}

func TestReadTruthRejectsInvalidLine(t *testing.T) {
	t.Parallel()
	in := "{\"transaction_id\":\"t1\",\"status\":202}\nnot json\n"
	_, err := ReadTruth(strings.NewReader(in))
	if err == nil || !strings.Contains(err.Error(), "truth line 2") {
		t.Fatalf("ReadTruth(%q) error = %v, want error naming line 2", in, err)
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 100)
	tests := []struct {
		in, want string
	}{
		{in: "short", want: "short"},
		{in: long, want: long[:80] + "..."},
	}
	for _, tt := range tests {
		if got := truncate([]byte(tt.in)); got != tt.want {
			t.Errorf("truncate(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
