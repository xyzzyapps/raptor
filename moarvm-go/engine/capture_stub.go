//go:build !windows && !unix

package moargo

func captureStdout(fn func() error) (string, error) {
	err := fn()
	return "", err
}
