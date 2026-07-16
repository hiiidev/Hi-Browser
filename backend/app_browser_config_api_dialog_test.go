package backend

import "testing"

func TestBrowserCoreImportDialogOptionsAvoidsMacOSWildcardCrash(t *testing.T) {
	options := browserCoreImportDialogOptions("darwin")
	if len(options.Filters) != 0 {
		t.Fatalf("macOS import filters = %#v, want none", options.Filters)
	}
}

func TestBrowserCoreImportDialogOptionsKeepsDesktopFilters(t *testing.T) {
	for _, goos := range []string{"windows", "linux"} {
		t.Run(goos, func(t *testing.T) {
			options := browserCoreImportDialogOptions(goos)
			if len(options.Filters) != 2 {
				t.Fatalf("%s import filters = %#v, want archive and catch-all filters", goos, options.Filters)
			}
		})
	}
}
