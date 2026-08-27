package bd

import "context"

// ReadyEntry is a ready bead's dispatch identity and bd priority.
type ReadyEntry struct {
	ID       string
	Priority int
}

// ReadyTasks returns ready non-epic, non-human bead IDs.
func (q *Coalescer) ReadyTasks(ctx context.Context) ([]string, error) {
	return q.queryIDs(ctx, ArgsReady())
}

// ReadyEntries returns ready non-epic, non-human beads with priority.
func (q *Coalescer) ReadyEntries(ctx context.Context) ([]ReadyEntry, error) {
	beads, err := q.queryBeads(ctx, ArgsReady())
	if err != nil {
		return nil, err
	}
	entries := make([]ReadyEntry, 0, len(beads))
	for _, bead := range beads {
		entries = append(entries, ReadyEntry{ID: bead.ID, Priority: bead.Priority})
	}
	return entries, nil
}

// InProgressTasks returns non-human in-progress task IDs.
func (q *Coalescer) InProgressTasks(ctx context.Context) ([]string, error) {
	return q.queryIDs(ctx, ArgsQuery(InProgressQuery(), false))
}

// OpenEpics returns open epic IDs.
func (q *Coalescer) OpenEpics(ctx context.Context) ([]string, error) {
	return q.queryIDs(ctx, ArgsQuery(OpenEpicsQuery(), false))
}

// EpicChildren returns IDs of an epic's direct children.
func (q *Coalescer) EpicChildren(ctx context.Context, epic string) ([]string, error) {
	return q.queryIDs(ctx, ArgsQuery(EpicChildrenQuery(epic), false))
}

// EpicOpenChildren returns IDs of an epic's open direct children.
func (q *Coalescer) EpicOpenChildren(ctx context.Context, epic string) ([]string, error) {
	return q.queryIDs(ctx, ArgsQuery(EpicOpenChildrenQuery(epic), false))
}

// DriftFixTasks returns non-human drift-fix bead IDs.
func (q *Coalescer) DriftFixTasks(ctx context.Context) ([]string, error) {
	return q.queryIDs(ctx, ArgsQuery(DriftFixQuery(), false))
}

func (q *Coalescer) queryIDs(ctx context.Context, args []string) ([]string, error) {
	result, err := q.Call(ctx, args...)
	if err != nil {
		return nil, err
	}
	return DecodeIDs(result.Stdout)
}

func (q *Coalescer) queryBeads(ctx context.Context, args []string) ([]Bead, error) {
	result, err := q.Call(ctx, args...)
	if err != nil {
		return nil, err
	}
	return DecodeBeads(result.Stdout)
}
