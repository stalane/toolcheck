package store

import "time"

type Tool struct {
	ID                int64
	Name              string
	Path              string
	RealPath          string
	Family            string
	Version           string
	Origin            string
	ManagedBy         string
	Risk              string
	Source            string
	Active            bool
	Writable          bool
	Updateable        bool
	LatestVersion     string
	LatestURL         string
	LatestPublishedAt *time.Time
	LatestSecurity    bool
	CheckedAt         *time.Time
}

type CheckRecord struct {
	ID         int64
	StartedAt  time.Time
	FinishedAt time.Time
	Status     string
	Tools      int
	Updates    int
	Error      string
}

type UpdateRecord struct {
	ID          int64
	ToolID      int64
	ToolName    string
	FromVersion string
	ToVersion   string
	Command     string
	Status      string
	Output      string
	StartedAt   time.Time
	FinishedAt  time.Time
}
