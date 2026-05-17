package workers

// SourcePullArgs is the job argument for a source pull task.
type SourcePullArgs struct {
	SourceName  string `json:"source_name"`
	TriggerType string `json:"trigger_type"`
}

func (SourcePullArgs) Kind() string { return "source_pull" }

// SourceProcessArgs is the job argument for a source processor task.
type SourceProcessArgs struct {
	SourceName string `json:"source_name"`
	PullRunID  string `json:"pull_run_id"`
}

func (SourceProcessArgs) Kind() string { return "source_process" }
