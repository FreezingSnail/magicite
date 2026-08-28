// Command fake-bd is a hermetic bd process double for parity tests.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/FreezingSnail/magicite/internal/testenv"
)

type dependency struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Status         string `json:"status"`
	DependencyType string `json:"dependency_type"`
}

type bead struct {
	ID                 string       `json:"id"`
	Title              string       `json:"title"`
	Description        string       `json:"description"`
	Design             string       `json:"design"`
	AcceptanceCriteria string       `json:"acceptance_criteria"`
	Status             string       `json:"status"`
	Priority           int          `json:"priority"`
	IssueType          string       `json:"issue_type"`
	Assignee           string       `json:"assignee"`
	Owner              string       `json:"owner"`
	Parent             string       `json:"parent"`
	CreatedAt          string       `json:"created_at"`
	UpdatedAt          string       `json:"updated_at"`
	StartedAt          string       `json:"started_at"`
	DependencyCount    int          `json:"dependency_count"`
	DependentCount     int          `json:"dependent_count"`
	CommentCount       int          `json:"comment_count"`
	Dependencies       []dependency `json:"dependencies"`
	Labels             []string     `json:"labels"`
	DeferredUntil      string       `json:"deferred_until,omitempty"`
	Comments           []string     `json:"comments,omitempty"`
	CloseReason        string       `json:"close_reason,omitempty"`
}

type failure struct {
	Subcommand string `json:"subcommand"`
	ExitCode   int    `json:"exit_code"`
	Stderr     string `json:"stderr"`
}

type store struct {
	Beads    []bead    `json:"beads"`
	Failures []failure `json:"failures,omitempty"`
	NextID   int       `json:"next_id"`
}

func main() {
	if err := testenv.Record(os.Getenv("MAGICITE_TRACE"), os.Args, workingDirectory()); err != nil {
		die(2, "record fake bd call: %v", err)
	}
	path := os.Getenv("MAGICITE_FAKE_BD_STORE")
	if path == "" {
		die(2, "MAGICITE_FAKE_BD_STORE is not set")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		die(2, "open fake bd store: %v", err)
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		die(2, "lock fake bd store: %v", err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	value, err := readStore(file)
	if err != nil {
		die(2, "read fake bd store: %v", err)
	}
	args := stripGlobal(os.Args[1:])
	subcommand := commandName(args)
	if failed, remaining := takeFailure(value.Failures, subcommand); failed != nil {
		value.Failures = remaining
		if err := writeStore(file, value); err != nil {
			die(2, "write fake bd store: %v", err)
		}
		fmt.Fprint(os.Stderr, failed.Stderr)
		os.Exit(failed.ExitCode)
	}

	output, mutate, err := execute(&value, args)
	if err != nil {
		die(2, "%v", err)
	}
	if mutate {
		if err := writeStore(file, value); err != nil {
			die(2, "write fake bd store: %v", err)
		}
	}
	fmt.Fprint(os.Stdout, output)
}

func workingDirectory() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return dir
}

func die(code int, format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(code)
}

func stripGlobal(args []string) []string {
	if len(args) >= 2 && args[0] == "-C" {
		return args[2:]
	}
	return args
}

func commandName(args []string) string {
	if len(args) == 0 {
		return ""
	}
	if (args[0] == "dep" || args[0] == "label") && len(args) > 1 {
		return args[0] + " " + args[1]
	}
	return args[0]
}

func takeFailure(failures []failure, subcommand string) (*failure, []failure) {
	for i, candidate := range failures {
		if candidate.Subcommand == subcommand {
			return &candidate, append(failures[:i:i], failures[i+1:]...)
		}
	}
	return nil, failures
}

func readStore(file *os.File) (store, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return store{}, err
	}
	contents, err := io.ReadAll(file)
	if err != nil {
		return store{}, err
	}
	if len(contents) == 0 {
		return store{NextID: 1}, nil
	}
	var value store
	if err := json.Unmarshal(contents, &value); err != nil {
		return store{}, err
	}
	if value.NextID < 1 {
		value.NextID = 1
	}
	return value, nil
}

func writeStore(file *os.File, value store) error {
	contents, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := file.Truncate(0); err != nil {
		return err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err := file.Write(contents); err != nil {
		return err
	}
	return file.Sync()
}

func execute(value *store, args []string) (string, bool, error) {
	if len(args) == 0 {
		return "", false, errors.New("unknown bd subcommand \"\"")
	}
	switch args[0] {
	case "ready":
		if !equal(args, []string{"ready", "--exclude-type", "epic", "--exclude-label", "human", "--json"}) {
			return "", false, usage(args[0])
		}
		result := make([]bead, 0)
		for _, item := range value.Beads {
			if item.Status == "open" && item.IssueType != "epic" && !hasLabel(item, "human") && item.DeferredUntil == "" {
				result = append(result, item)
			}
		}
		return jsonArray(result)
	case "list":
		all, ok := allFlag(args, "list")
		if !ok {
			return "", false, usage(args[0])
		}
		return jsonArray(filterStatus(value.Beads, all))
	case "show":
		if len(args) != 3 || args[2] != "--json" {
			return "", false, usage(args[0])
		}
		item, ok := find(value.Beads, args[1])
		if !ok {
			return "[]", false, nil
		}
		return jsonArray([]bead{item})
	case "dep":
		return dependencies(value, args)
	case "label":
		return labels(value, args)
	case "query":
		return query(value, args)
	case "comment":
		return comment(value, args)
	case "close":
		return closeBead(value, args)
	case "create":
		return create(value, args)
	case "update":
		return update(value, args)
	default:
		return "", false, fmt.Errorf("unknown bd subcommand %q", args[0])
	}
}

func dependencies(value *store, args []string) (string, bool, error) {
	if len(args) == 4 && equal(args[:2], []string{"dep", "list"}) && args[3] == "--json" {
		item, ok := find(value.Beads, args[2])
		if !ok {
			return "[]", false, nil
		}
		return jsonArray(item.Dependencies)
	}
	if len(args) == 4 && equal(args[:2], []string{"dep", "add"}) {
		item, ok := findPointer(value.Beads, args[2])
		if !ok {
			return "", false, fmt.Errorf("bead %q not found", args[2])
		}
		dependencyBead, exists := find(value.Beads, args[3])
		if !exists {
			return "", false, fmt.Errorf("bead %q not found", args[3])
		}
		item.Dependencies = append(item.Dependencies, dependency{ID: dependencyBead.ID, Title: dependencyBead.Title, Status: dependencyBead.Status, DependencyType: "blocks"})
		item.DependencyCount = len(item.Dependencies)
		return "", true, nil
	}
	return "", false, usage("dep")
}

func labels(value *store, args []string) (string, bool, error) {
	if len(args) == 4 && equal(args[:2], []string{"label", "list"}) && args[3] == "--json" {
		item, ok := find(value.Beads, args[2])
		if !ok {
			return "[]", false, nil
		}
		return jsonArray(item.Labels)
	}
	if len(args) == 4 && equal(args[:2], []string{"label", "add"}) {
		item, ok := findPointer(value.Beads, args[2])
		if !ok {
			return "", false, fmt.Errorf("bead %q not found", args[2])
		}
		if !hasLabel(*item, args[3]) {
			item.Labels = append(item.Labels, args[3])
		}
		return "", true, nil
	}
	if len(args) == 4 && equal(args[:2], []string{"label", "remove"}) {
		item, ok := findPointer(value.Beads, args[2])
		if !ok {
			return "", false, fmt.Errorf("bead %q not found", args[2])
		}
		item.Labels = removeLabel(item.Labels, args[3])
		return "", true, nil
	}
	return "", false, usage("label")
}

func query(value *store, args []string) (string, bool, error) {
	if len(args) != 3 && len(args) != 4 {
		return "", false, usage("query")
	}
	if args[2] != "--json" || (len(args) == 4 && args[3] != "--all") {
		return "", false, usage("query")
	}
	all := len(args) == 4
	result := make([]bead, 0)
	for _, item := range filterStatus(value.Beads, all) {
		if matchesQuery(item, args[1]) {
			result = append(result, item)
		}
	}
	return jsonArray(result)
}

func comment(value *store, args []string) (string, bool, error) {
	if len(args) != 3 && len(args) != 4 {
		return "", false, usage("comment")
	}
	item, ok := findPointer(value.Beads, args[1])
	if !ok {
		return "", false, fmt.Errorf("bead %q not found", args[1])
	}
	text := args[2]
	if len(args) == 4 {
		if args[2] != "--file" {
			return "", false, usage("comment")
		}
		contents, err := os.ReadFile(args[3])
		if err != nil {
			return "", false, fmt.Errorf("read comment: %w", err)
		}
		text = string(contents)
	}
	item.Comments = append(item.Comments, text)
	item.CommentCount = len(item.Comments)
	return "", true, nil
}

func closeBead(value *store, args []string) (string, bool, error) {
	if len(args) != 4 || args[2] != "--reason-file" {
		return "", false, usage("close")
	}
	item, ok := findPointer(value.Beads, args[1])
	if !ok {
		return "", false, fmt.Errorf("bead %q not found", args[1])
	}
	reason, err := os.ReadFile(args[3])
	if err != nil {
		return "", false, fmt.Errorf("read close reason: %w", err)
	}
	item.Status, item.CloseReason = "closed", string(reason)
	return "", true, nil
}

func create(value *store, args []string) (string, bool, error) {
	if len(args) < 5 || args[2] != "--type" || args[4] != "--silent" {
		return "", false, usage("create")
	}
	item := bead{ID: nextID(value), Title: args[1], IssueType: args[3], Status: "open"}
	for i := 5; i < len(args); {
		if i+1 >= len(args) {
			return "", false, usage("create")
		}
		flag, argument := args[i], args[i+1]
		switch flag {
		case "--parent":
			item.Parent = argument
		case "--body-file":
			text, err := os.ReadFile(argument)
			if err != nil {
				return "", false, err
			}
			item.Description = string(text)
		case "--design-file":
			text, err := os.ReadFile(argument)
			if err != nil {
				return "", false, err
			}
			item.Design = string(text)
		case "--acceptance":
			item.AcceptanceCriteria = argument
		case "--labels":
			item.Labels = splitLabels(argument)
		case "--priority":
			priority, err := strconv.Atoi(argument)
			if err != nil {
				return "", false, usage("create")
			}
			item.Priority = priority
		default:
			return "", false, usage("create")
		}
		i += 2
	}
	value.Beads = append(value.Beads, item)
	return item.ID, true, nil
}

func update(value *store, args []string) (string, bool, error) {
	if len(args) < 2 {
		return "", false, usage("update")
	}
	item, ok := findPointer(value.Beads, args[1])
	if !ok {
		return "", false, fmt.Errorf("bead %q not found", args[1])
	}
	for i := 2; i < len(args); {
		flag := args[i]
		if flag == "--claim" {
			item.Status = "in_progress"
			i++
			continue
		}
		if i+1 >= len(args) {
			return "", false, usage("update")
		}
		argument := args[i+1]
		switch flag {
		case "--status":
			item.Status = argument
		case "--assignee":
			item.Assignee = argument
		case "--body-file":
			text, err := os.ReadFile(argument)
			if err != nil {
				return "", false, err
			}
			item.Description = string(text)
		case "--design-file":
			text, err := os.ReadFile(argument)
			if err != nil {
				return "", false, err
			}
			item.Design = string(text)
		case "--acceptance":
			item.AcceptanceCriteria = argument
		case "--add-label":
			if !hasLabel(*item, argument) {
				item.Labels = append(item.Labels, argument)
			}
		case "--remove-label":
			item.Labels = removeLabel(item.Labels, argument)
		case "--defer":
			item.DeferredUntil = argument
		default:
			return "", false, usage("update")
		}
		i += 2
	}
	return "", true, nil
}

func allFlag(args []string, command string) (bool, bool) {
	if len(args) == 2 && args[0] == command && args[1] == "--json" {
		return false, true
	}
	if len(args) == 3 && args[0] == command && args[1] == "--json" && args[2] == "--all" {
		return true, true
	}
	return false, false
}
func usage(command string) error { return fmt.Errorf("invalid bd %s arguments", command) }
func equal(a, b []string) bool {
	return len(a) == len(b) && strings.Join(a, "\x00") == strings.Join(b, "\x00")
}
func find(items []bead, id string) (bead, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return bead{}, false
}
func findPointer(items []bead, id string) (*bead, bool) {
	for i := range items {
		if items[i].ID == id {
			return &items[i], true
		}
	}
	return nil, false
}
func hasLabel(item bead, label string) bool {
	for _, value := range item.Labels {
		if value == label {
			return true
		}
	}
	return false
}
func removeLabel(labels []string, label string) []string {
	result := labels[:0]
	for _, value := range labels {
		if value != label {
			result = append(result, value)
		}
	}
	return result
}
func filterStatus(items []bead, all bool) []bead {
	result := make([]bead, 0, len(items))
	for _, item := range items {
		if all || item.Status != "closed" {
			result = append(result, item)
		}
	}
	return result
}
func splitLabels(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
}
func matchesQuery(item bead, query string) bool {
	for _, term := range strings.Split(query, " AND ") {
		negated := strings.HasPrefix(term, "NOT ")
		term = strings.TrimPrefix(term, "NOT ")
		key, value, ok := strings.Cut(term, "=")
		if !ok {
			continue
		}
		matched := (key == "status" && item.Status == value) || (key == "type" && item.IssueType == value) || (key == "parent" && item.Parent == value) || (key == "label" && hasLabel(item, value))
		if negated {
			matched = !matched
		}
		if !matched {
			return false
		}
	}
	return true
}
func nextID(value *store) string {
	for {
		id := fmt.Sprintf("bd-%d", value.NextID)
		value.NextID++
		if _, exists := find(value.Beads, id); !exists {
			return id
		}
	}
}
func jsonArray(value any) (string, bool, error) {
	contents, err := json.Marshal(value)
	if err != nil {
		return "", false, err
	}
	return string(contents), false, nil
}
