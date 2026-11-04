// The Device Detection examples. They aren't published. Like a customer's code,
// they use the published module; ci/create-workspace.ps1 makes them use the one
// in this repository.
module github.com/51Degrees/51degrees-go/examples/device-detection

go 1.19

require (
	github.com/51Degrees/51degrees-go/device-detection/v4 v4.5.169
	gopkg.in/yaml.v3 v3.0.1
)

require github.com/51Degrees/51degrees-go/common/v4 v4.5.169 // indirect
