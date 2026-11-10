package gitlog

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	out := "" +
		"*   \x1faaaa\x1fbbbb cccc\x1falice\x1f2 hours ago\x1fHEAD -> main, tag: v1\x1fMerge branch 'feature'\n" +
		"|\\\n" +
		"| * \x1fcccc\x1fdddd\x1fbob\x1f3 hours ago\x1f\x1ffeature work\n" +
		"|/\n" +
		"* \x1fdddd\x1f\x1falice\x1f1 day ago\x1f\x1finitial\n"

	got := Parse(out)
	if len(got) != 3 {
		t.Fatalf("want 3 commits, got %d", len(got))
	}
	if got[0].Hash != "aaaa" || got[0].Graph != "*   " {
		t.Errorf("commit 0 = %+v", got[0])
	}
	if !reflect.DeepEqual(got[0].Parents, []string{"bbbb", "cccc"}) {
		t.Errorf("parents = %v", got[0].Parents)
	}
	if !reflect.DeepEqual(got[0].Tail, []string{"|\\"}) {
		t.Errorf("tail = %v", got[0].Tail)
	}
	if got[1].Graph != "| * " || got[1].Subject != "feature work" {
		t.Errorf("commit 1 = %+v", got[1])
	}
	if len(got[2].Parents) != 0 || got[2].Graph != "* " {
		t.Errorf("commit 2 = %+v", got[2])
	}
}

func TestIsGraphOnly(t *testing.T) {
	for _, s := range []string{"|\\", "|/", "| |", "* |", "  "} {
		if !isGraphOnly(s) {
			t.Errorf("%q should be graph only", s)
		}
	}
	for _, s := range []string{"| * abc", "text", "|\\x"} {
		if isGraphOnly(s) {
			t.Errorf("%q should not be graph only", s)
		}
	}
}
