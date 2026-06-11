package core

import "strings"

// lower normalizes a string key to lower case. All string keys in the Pipeline
// API are compared case-insensitively.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/access-to-results.md
func lower(s string) string { return strings.ToLower(s) }
