//go:build !windows

package datasyncui

type nativeTrayCallbacks struct {
	Show        func()
	RequestExit func()
}

type nativeTray struct{}

func startNativeTray(callbacks nativeTrayCallbacks) (*nativeTray, error) {
	return &nativeTray{}, nil
}

func (t *nativeTray) Stop() {}
