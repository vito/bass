package bass

// A workload is a command to be run with a runtime.
type Workload struct {
	// Deterministic ID for the workload. Used for caching.
	//
	// Can be referenced by later workloads.
	ID string `json:"id,omitempty"`

	// Platform is an object used to select an appropriate runtime to run the
	// command.
	Platform Object `json:"platform,omitempty"`

	// All workloads have the following fields, which are Values so that they may
	// be resolved late by the runtime receiving the workload.
	Path     Value  `json:"path"`
	Image    Value  `json:"image,omitempty"`
	Insecure bool   `json:"insecure,omitempty"`
	Args     List   `json:"args,omitempty"`
	Stdin    List   `json:"stdin,omitempty"`
	Env      Object `json:"env,omitempty"`
	Dir      Path   `json:"dir,omitempty"`

	// read response from stdout
	ResponseFromStdout bool `json:"response_from_stdout,omitempty"`

	// file to read response payload from
	ResponseFromFile *FilePath `json:"response_from_file,omitempty"`

	// send the exit code as a single response
	ResponseFromExitCode bool `json:"response_from_exit_code,omitempty"`
}
