package bass

// Apply is a List which Calls its first value against the remaining values.
//
// If the List is Empty, or if the first value is not Callable, an error is
// returned.
type Apply List
