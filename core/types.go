package core

// JavaScript is a Property value type identifying a string that contains a
// JavaScript snippet intended to be executed on the client device. In JSON it
// is represented as a string. It is used by the JSON Builder to identify
// JavaScript Properties and by the web integration's client-side execution.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/properties.md#the-javascript-type
type JavaScript string

// String returns the snippet text.
func (j JavaScript) String() string { return string(j) }
