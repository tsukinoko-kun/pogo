package serve

import (
	"fmt"
	"github.com/tsukinoko-kun/pogo/text"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/devsisters/go-diff3"
)

func TestMergeTextChangesSingle(t *testing.T) {
	base := text.NewText("foo\nbar\nbaz\n")
	others := []textFileOtherChange{
		{
			textContent: text.NewText("foo\nbarrrr\nbaz\n"),
			changeName:  "barrrr",
		},
	}
	res, hasConflicts, err := mergeTextChanges(base, others)
	if err != nil {
		t.Fatal(err)
	}
	if hasConflicts {
		t.Fatal("merge should not have conflicts")
	}
	if res.String() != "foo\nbarrrr\nbaz\n" {
		t.Fatalf("merge result should be 'foo\nbarrrr\nbaz\n', got %s", res.String())
	}
}

func TestMergeTextChangesConflict(t *testing.T) {
	base := text.NewText("foo\nbar\nbaz\n")
	others := []textFileOtherChange{
		{
			textContent: text.NewText("foo\nbarrrr\nbaz\n"),
			changeName:  "A",
		},
		{
			textContent: text.NewText("foo\nlorem ipsum\nbaz\n"),
			changeName:  "B",
		},
	}
	res, hasConflicts, err := mergeTextChanges(base, others)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConflicts {
		t.Fatal("merge should have conflicts")
	}
	expected := "foo\n<<<<<<<<< A\nbarrrr\n=========\nlorem ipsum\n>>>>>>>>> B\nbaz"
	if res.String() != expected {
		t.Fatalf("merge result should be:\n%q\n\n\ngot:\n%q", expected, res.String())
	}
}

func TestDiff3Library(t *testing.T) {
	b, err := os.ReadFile("merge.go")
	if err != nil {
		t.Fatal(err)
	}

	base := string(b)

	lineA := "some random line that is not in the original"
	lineB := "some other line that is surely in the original"

	lines := strings.Split(base, "\n")
	i := len(lines) / 2

	lines[i] = lineA
	changeA := strings.Join(lines, "\n") + "\n"

	lines[i] = lineB
	changeB := strings.Join(lines, "\n") + "\n"

	expectedDiff := strings.Builder{}
	for j := 0; j < i; j++ {
		expectedDiff.WriteString(lines[j])
		expectedDiff.WriteString("\n")
	}
	expectedDiff.WriteString("<<<<<<<<< A\n")
	expectedDiff.WriteString(lineA)
	expectedDiff.WriteString("\n=========\n")
	expectedDiff.WriteString(lineB)
	expectedDiff.WriteString("\n>>>>>>>>> B\n")
	for j := i + 1; j < len(lines); j++ {
		if j > i+1 {
			expectedDiff.WriteString("\n")
		}
		expectedDiff.WriteString(lines[j])
	}

	var diff string
	if err := func() error {
		defer func() {
			if r := recover(); r != nil {
				diff = fmt.Sprintf("diff3 panicked: %v", r)
			}
		}()
		res, err := diff3.Merge(strings.NewReader(changeA), strings.NewReader(base), strings.NewReader(changeB), true, "A", "B")
		if err != nil {
			return err
		}
		diffB, _ := io.ReadAll(res.Result)
		diff = string(diffB)
		return nil
	}(); err != nil {
		t.Fatal(err)
	}
	if diff != expectedDiff.String() {
		t.Fatalf("diff should be:\n%q\n\n\ngot:\n%q", expectedDiff.String(), diff)
	}
}
