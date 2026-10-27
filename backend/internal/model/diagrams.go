package model

// Position is where a table sits on the canvas.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Diagram is the `diagrams` row as every layer sees it. An empty FolderID or
// ShareToken means "none".
type Diagram struct {
	Guid       string              `json:"guid"`
	Name       string              `json:"name"`
	DBML       string              `json:"dbml"`
	Positions  map[string]Position `json:"positions"`
	FolderID   string              `json:"folder_id"`
	ShareToken string              `json:"share_token"`
	OwnerID    string              `json:"owner_id"`
	OwnerName  string              `json:"owner_name"`
	// Role is the CALLER's role, filled in by the service; it is not a column.
	Role          Role   `json:"role"`
	Revision      int64  `json:"revision"`
	UpdatedByName string `json:"updated_by_name"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// DiagramItem is a Diagram without the large columns, for lists.
type DiagramItem struct {
	Guid       string `json:"guid"`
	Name       string `json:"name"`
	FolderID   string `json:"folder_id"`
	ShareToken string `json:"share_token"`
	OwnerID    string `json:"owner_id"`
	OwnerName  string `json:"owner_name"`
	Role       Role   `json:"role"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// SharedDiagram is all a public link reveals.
type SharedDiagram struct {
	Name      string              `json:"name"`
	DBML      string              `json:"dbml"`
	Positions map[string]Position `json:"positions"`
}

// Folder nests: ParentID is "" for a top-level folder, and also when the parent
// is one the caller cannot see (a folder shared on its own).
type Folder struct {
	Guid      string `json:"guid"`
	Name      string `json:"name"`
	ParentID  string `json:"parent_id"`
	OwnerID   string `json:"owner_id"`
	OwnerName string `json:"owner_name"`
	Role      Role   `json:"role"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// VersionItem is a Version without its large columns, for lists. An empty
// Label marks an automatic snapshot.
type VersionItem struct {
	Guid      string `json:"guid"`
	DiagramID string `json:"diagram_id"`
	Label     string `json:"label"`
	CreatedAt string `json:"created_at"`
}

// Version is an immutable snapshot of a diagram's DBML and layout.
type Version struct {
	Guid      string              `json:"guid"`
	DiagramID string              `json:"diagram_id"`
	Label     string              `json:"label"`
	DBML      string              `json:"dbml"`
	Positions map[string]Position `json:"positions"`
	CreatedAt string              `json:"created_at"`
}
