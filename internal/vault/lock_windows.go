//go:build windows

package vault

func AcquireExclusive(_ string) (func(), error) {
	// Windows: no flock; relies on daemon serialization
	return func() {}, nil
}

func AcquireShared(_ string) (func(), error) {
	return func() {}, nil
}
