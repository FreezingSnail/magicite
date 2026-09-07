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
	CreatedBy          string       `json:"created_by"`
	UpdatedAt          string       `json:"updated_at"`
	StartedAt          string       `json:"started_at"`
	ClosedAt           string       `json:"closed_at"`
	CloseReason        string       `json:"close_reason"`
	DeferredUntil      string       `json:"deferred_until"`
	DependencyCount    int          `json:"dependency_count"`
	DependentCount     int          `json:"dependent_count"`
	CommentCount       int          `json:"comment_count"`
	Labels             []string     `json:"labels"`
	Comments           []string     `json:"comments"`
	Dependencies       []Dependency `json:"dependencies"`
}

// Dependency is an edge or summary reported by bd for a bead.
type Dependency struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Status         string `json:"status"`
	DependencyType string `json:"dependency_type"`
	IssueID        string `json:"issue_id"`
	DependsOnID    string `json:"depends_on_id"`
	Type           string `json:"type"`
	CreatedAt      string `json:"created_at"`
	CreatedBy      string `json:"created_by"`
	Metadata       string `json:"metadata"`
}

// Envelope is bd's JSON error response format.
type Envelope struct {
	Error         string `json:"error"`
	SchemaVersion int    `json:"schema_version"`
}
