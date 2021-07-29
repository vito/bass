package bass

// A workload is a command to be run with a runtime.
type Workload struct {
	// Deterministic ID for the workload. Used for caching.
	//
	// Can be referenced by later workloads.
	ID string `bass:"id" json:"id" optional:"true"`

	// Platform is an object used to select an appropriate runtime to run the
	// command.
	Platform Object `bass:"platform" json:"platform" optional:"true"`

	// All workloads have the following fields, which are Values so that they may
	// be resolved late by the runtime receiving the workload.
	Path     Value  `bass:"path" json:"path"`
	Image    Value  `bass:"image" optional:"true" json:"image,omitempty"`
	Insecure bool   `bass:"insecure" optional:"true" json:"insecure,omitempty"`
	Args     List   `bass:"args"  optional:"true" json:"args,omitempty"`
	Stdin    List   `bass:"stdin" optional:"true" json:"stdin,omitempty"`
	Env      Object `bass:"env"   optional:"true" json:"env,omitempty"`
	Dir      Path   `bass:"dir"   optional:"true" json:"dir,omitempty"`

	// read response from stdout
	ResponseFromStdout bool `bass:"response_from_stdout" optional:"true" json:"response_from_stdout,omitempty"`

	// file to read response payload from
	ResponseFromFile *FilePath `bass:"response_from_file" optional:"true" json:"response_from_file,omitempty"`

	// send the exit code as a single response
	ResponseFromExitCode bool `bass:"response_from_exit_code" optional:"true" json:"response_from_exit_code,omitempty"`
}
