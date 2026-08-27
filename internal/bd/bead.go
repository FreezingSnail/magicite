package bd

// Bead is a bd issue record as emitted by its JSON commands.
type Bead struct {
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
	Dependencies       []Dependency `json:"dependencies"`
}

// Dependency is an edge reported by bd for a bead.
type Dependency struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Status         string `json:"status"`
	DependencyType string `json:"dependency_type"`
}

// Envelope is bd's JSON error response format.
type Envelope struct {
	Error         string `json:"error"`
	SchemaVersion int    `json:"schema_version"`
}
