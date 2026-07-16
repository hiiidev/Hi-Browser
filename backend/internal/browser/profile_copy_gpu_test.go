package browser

import (
	"reflect"
	"testing"
)

func TestApplyCopyAutomationTargetsRenderIsGPUNeutral(t *testing.T) {
	base := []string{
		"--fingerprint-canvas-noise=true",
		"--fingerprint-audio-noise=true",
		"--fingerprint-webgl-vendor=Apple",
		"--fingerprint-webgl-renderer=Apple M2",
	}
	defaults := []string{
		"--fingerprint-canvas-noise=false",
		"--fingerprint-audio-noise=false",
		"--fingerprint-webgl-vendor=Intel",
		"--fingerprint-webgl-renderer=Intel Iris",
	}

	got := applyCopyAutomationTargets(base, defaults, []string{copyAutomationTargetRender})
	want := []string{
		"--fingerprint-canvas-noise=false",
		"--fingerprint-audio-noise=false",
		"--fingerprint-webgl-vendor=Apple",
		"--fingerprint-webgl-renderer=Apple M2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("applyCopyAutomationTargets() = %#v, want %#v", got, want)
	}
}

func TestRegularCopyFingerprintArgsPreservesLegacyGPUData(t *testing.T) {
	source := []string{"--fingerprint=7", "--fingerprint-webgl-vendor=Apple", "--fingerprint-webgl-renderer=Apple M2"}
	got := append([]string{}, source...)
	if !reflect.DeepEqual(got, source) {
		t.Fatalf("regular copy args = %#v, want %#v", got, source)
	}
}
