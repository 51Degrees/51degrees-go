// The IP Intelligence examples. They aren't published. Like a customer's code,
// they use the published module; ci/create-workspace.ps1 makes them use the one
// in this repository.
module github.com/51Degrees/51degrees-go/examples/ip-intelligence

go 1.21.0

require (
	github.com/51Degrees/51degrees-go/ip-intelligence/v4 v4.5.168
	github.com/goccy/go-yaml v1.19.2
)

require github.com/51Degrees/51degrees-go/common/v4 v4.5.168 // indirect
