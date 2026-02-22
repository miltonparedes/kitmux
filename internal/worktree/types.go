package worktree

// Worktree matches the JSON output of `wt list --format=json`.
type Worktree struct {
	Branch      string      `json:"branch"`
	Path        string      `json:"path"`
	Kind        string      `json:"kind"`
	Commit      Commit      `json:"commit"`
	WorkingTree WorkingTree `json:"working_tree"`
	MainState   string      `json:"main_state"`
	Remote      Remote      `json:"remote"`
	IsMain      bool        `json:"is_main"`
	IsCurrent   bool        `json:"is_current"`
	IsPrevious  bool        `json:"is_previous"`
	Symbols     string      `json:"symbols"`
}

type Commit struct {
	SHA       string `json:"sha"`
	ShortSHA  string `json:"short_sha"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

type WorkingTree struct {
	Staged    bool `json:"staged"`
	Modified  bool `json:"modified"`
	Untracked bool `json:"untracked"`
	Renamed   bool `json:"renamed"`
	Deleted   bool `json:"deleted"`
	Diff      Diff `json:"diff"`
}

type Diff struct {
	Added   int `json:"added"`
	Deleted int `json:"deleted"`
}

type Remote struct {
	Name   string `json:"name"`
	Branch string `json:"branch"`
	Ahead  int    `json:"ahead"`
	Behind int    `json:"behind"`
}
