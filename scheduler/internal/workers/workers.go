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

// DomainImportArgs are the arguments for a CSV domain import River job.
type DomainImportArgs struct {
	BatchID  string `json:"batch_id"`
	CsvS3Key string `json:"csv_s3_key"`
}

func (DomainImportArgs) Kind() string { return "domain_import" }
