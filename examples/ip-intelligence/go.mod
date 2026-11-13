// The IP Intelligence examples. They aren't published. Like a customer's code,
// they use the published module; ci/create-workspace.ps1 makes them use the one
// in this repository.
module github.com/51Degrees/51degrees-go/examples/ip-intelligence

go 1.22

require (
	github.com/51Degrees/51degrees-go/ip-intelligence/v4 v4.5.173
	github.com/51Degrees/51degrees-go/pipeline/v4 v4.5.173
	github.com/goccy/go-yaml v1.19.2
)

require (
	github.com/51Degrees/51degrees-go/common/v4 v4.5.173 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
)
