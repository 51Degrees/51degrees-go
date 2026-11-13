// The Pipeline examples. They aren't published. Copied out of the repository
// they use the published module, as a customer's code does. In a clone, the
// go.work at the root makes them use the one in this repository.
module github.com/51Degrees/51degrees-go/examples/pipeline

go 1.22

require github.com/51Degrees/51degrees-go/pipeline/v4 v4.5.171

require (
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
)
