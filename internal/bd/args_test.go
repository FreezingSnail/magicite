package bd

import (
	"reflect"
	"testing"
)

func TestArgsReady(t *testing.T) {
	want := []string{"ready", "--exclude-type", "epic", "--exclude-label", "human", "--json"}
	assertArgs(t, ArgsReady(), want)
}

func TestArgsListAndQuery(t *testing.T) {
	assertArgs(t, ArgsList(false), []string{"list", "--json"})
	assertArgs(t, ArgsList(true), []string{"list", "--json", "--all"})
	assertArgs(t, ArgsQuery("status=open", false), []string{"query", "status=open", "--json"})
	assertArgs(t, ArgsQuery("status=open", true), []string{"query", "status=open", "--json", "--all"})
}

func TestArgsShowDependenciesAndLabels(t *testing.T) {
	assertArgs(t, ArgsShow("bd-1"), []string{"show", "bd-1", "--json"})
	assertArgs(t, ArgsDepList("bd-1"), []string{"dep", "list", "bd-1", "--json"})
	assertArgs(t, ArgsDepAdd("bd-1", "bd-2"), []string{"dep", "add", "bd-1", "bd-2"})
	assertArgs(t, ArgsLabelList("bd-1"), []string{"label", "list", "bd-1", "--json"})
	assertArgs(t, ArgsLabelAdd("bd-1", "human"), []string{"label", "add", "bd-1", "human"})
	assertArgs(t, ArgsLabelRemove("bd-1", "human"), []string{"label", "remove", "bd-1", "human"})
}

func TestArgsCreateOmitsEmptyFieldsAndDefaultsType(t *testing.T) {
	assertArgs(t, ArgsCreate(CreateArgs{Title: "title"}), []string{
		"create", "title", "--type", "task", "--silent",
	})
	assertArgs(t, ArgsCreate(CreateArgs{
		Title: "title", Type: "epic", Parent: "bd-0", BodyFile: "body.md",
		DesignFile: "design.md", Acceptance: "one line", Labels: []string{"a", "b"}, Priority: "2",
	}), []string{
		"create", "title", "--type", "epic", "--silent",
		"--parent", "bd-0", "--body-file", "body.md", "--design-file", "design.md",
		"--acceptance", "one line", "--labels", "a,b", "--priority", "2",
	})
}

func TestArgsUpdateOmitsEmptyFieldsAndRepeatsLabels(t *testing.T) {
	assertArgs(t, ArgsUpdate("bd-1", UpdateArgs{}), []string{"update", "bd-1"})
	assertArgs(t, ArgsUpdate("bd-1", UpdateArgs{
		Status: "in_progress", Assignee: "worker", Claim: true, BodyFile: "body.md",
		DesignFile: "design.md", Acceptance: "done", AddLabels: []string{"a", "b"},
		RemoveLabels: []string{"c", "d"}, Defer: "tomorrow",
	}), []string{
		"update", "bd-1", "--status", "in_progress", "--assignee", "worker", "--claim",
		"--body-file", "body.md", "--design-file", "design.md", "--acceptance", "done",
		"--add-label", "a", "--add-label", "b", "--remove-label", "c", "--remove-label", "d",
		"--defer", "tomorrow",
	})
}

func TestArgsCloseCommentAndDefer(t *testing.T) {
	assertArgs(t, ArgsClose("bd-1", "reason.md"), []string{"close", "bd-1", "--reason-file", "reason.md"})
	assertArgs(t, ArgsComment("bd-1", "comment.md"), []string{"comment", "bd-1", "--file", "comment.md"})
	assertArgs(t, ArgsDefer("bd-1", "tomorrow"), []string{"update", "bd-1", "--defer", "tomorrow"})
	assertArgs(t, ArgsUndefer("bd-1"), []string{"update", "bd-1", "--defer", ""})
}

func TestQueryVocabulary(t *testing.T) {
	if HumanLabel != "human" {
		t.Fatalf("HumanLabel = %q", HumanLabel)
	}
	checks := map[string]string{
		NotHumanQuery("status=open"):    "status=open AND NOT label=human",
		InProgressQuery():               "status=in_progress AND type=task AND NOT label=human",
		DriftFixQuery():                 "label=drift-fix AND NOT label=human",
		OpenEpicsQuery():                "status=open AND type=epic",
		EpicChildrenQuery("epic-1"):     "parent=epic-1",
		EpicOpenChildrenQuery("epic-1"): "parent=epic-1 AND status=open",
	}
	for got, want := range checks {
		if got != want {
			t.Errorf("query = %q, want %q", got, want)
		}
	}
}

func TestIsHumanAndDifficultyFromLabels(t *testing.T) {
	for _, labels := range [][]string{{"human"}, {" HUMAN "}, {"HuMaN", "other"}} {
		if !IsHuman(labels) {
			t.Errorf("IsHuman(%q) = false", labels)
		}
	}
	if IsHuman([]string{"human-ish", "person"}) {
		t.Error("IsHuman matched a non-human label")
	}
	checks := []struct {
		labels []string
		want   string
	}{
		{nil, ""},
		{[]string{"difficulty:low"}, "low"},
		{[]string{" DIFFICULTY:LOW "}, "low"},
		{[]string{"difficulty:low", "difficulty:high"}, "high"},
		{[]string{"difficulty:high", "difficulty:low"}, "high"},
		{[]string{"difficulty:medium"}, ""},
	}
	for _, check := range checks {
		if got := DifficultyFromLabels(check.labels); got != check.want {
			t.Errorf("DifficultyFromLabels(%q) = %q, want %q", check.labels, got, check.want)
		}
	}
}

func assertArgs(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %#v, want %#v", got, want)
	}
}
