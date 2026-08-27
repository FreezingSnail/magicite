package stamp

import "strings"

const Prefix = "Magicite-"

const (
	KeyModel      = Prefix + "Model"
	KeyBackend    = Prefix + "Backend"
	KeyDifficulty = Prefix + "Difficulty"
	KeyEffort     = Prefix + "Effort"
	KeyAgent      = Prefix + "Agent"
	KeyRepo       = Prefix + "Repo"
	KeySeat       = Prefix + "Seat"
	KeyTask       = Prefix + "Task"
	KeyHarness    = Prefix + "Harness"
	KeyHarnessRev = Prefix + "Harness-Rev"
)

type Trailer struct {
	Key   string
	Value string
}

type Stamp struct {
	Model, Backend, Difficulty, Effort, Agent, Repo, Seat, Task, Harness, HarnessRev string
}

var keys = []string{
	KeyModel,
	KeyBackend,
	KeyDifficulty,
	KeyEffort,
	KeyAgent,
	KeyRepo,
	KeySeat,
	KeyTask,
	KeyHarness,
	KeyHarnessRev,
}

func Keys() []string {
	return append([]string(nil), keys...)
}

func Sanitize(value string) string {
	value = strings.NewReplacer("\r", " ", "\n", " ", ":", " ").Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func (s Stamp) Trailers() []Trailer {
	values := [...]string{
		s.Model,
		s.Backend,
		s.Difficulty,
		s.Effort,
		s.Agent,
		s.Repo,
		s.Seat,
		s.Task,
		s.Harness,
		s.HarnessRev,
	}
	trailers := make([]Trailer, 0, len(keys))
	for index, value := range values {
		if value = Sanitize(value); value != "" {
			trailers = append(trailers, Trailer{Key: keys[index], Value: value})
		}
	}
	return trailers
}
