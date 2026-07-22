package clipboard

import (
	"errors"
	"reflect"
	"testing"
)

func TestPlatformProviderOrderAndArguments(t *testing.T) {
	var attempts []string
	lookPath := func(name string) (string, error) {
		attempts = append(attempts, name)
		if name == "xclip" {
			return "/test/xclip", nil
		}
		return "", errors.New("not found")
	}
	name, args, err := platformProvider("linux", lookPath)
	if err != nil {
		t.Fatal(err)
	}
	if name != "/test/xclip" || !reflect.DeepEqual(args, []string{"-selection", "clipboard"}) {
		t.Fatalf("provider = %q %q", name, args)
	}
	if want := []string{"wl-copy", "xclip"}; !reflect.DeepEqual(attempts, want) {
		t.Fatalf("provider attempts = %q, want %q", attempts, want)
	}
}

func TestPlatformProviderDarwinAndUnsupported(t *testing.T) {
	name, args, err := platformProvider("darwin", func(name string) (string, error) {
		return "/test/" + name, nil
	})
	if err != nil || name != "/test/pbcopy" || len(args) != 0 {
		t.Fatalf("darwin provider = %q %q, %v", name, args, err)
	}
	if _, _, err := platformProvider("plan9", func(string) (string, error) { return "", nil }); err == nil {
		t.Fatal("unsupported platform did not return an error")
	}
}
