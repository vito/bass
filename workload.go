package bass

// A workload is a black-box object containing fields interpreted by its
// runtime.
type Workload struct {
	// Platform is an object used to select an appropriate runtime to run the
	// command.
	Platform Object `bass:"platform"`

	// Command is a black-box object sent directly to the runtime associated to
	// the platform.
	Command Value `bass:"command"`
}
