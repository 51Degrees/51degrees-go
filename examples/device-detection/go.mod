// The Device Detection examples. They aren't published. Copied out of the
// repository they use the published module, as a customer's code does. In a
// clone, the go.work at the root makes them use the one in this repository.
module github.com/51Degrees/51degrees-go/examples/device-detection

go 1.22

require (
	github.com/51Degrees/51degrees-go/device-detection/v4 v4.6.2
	github.com/51Degrees/51degrees-go/pipeline/v4 v4.6.2
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/51Degrees/51degrees-go/common/v4 v4.6.2 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
)
