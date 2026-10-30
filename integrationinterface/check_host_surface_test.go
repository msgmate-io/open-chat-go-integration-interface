// Tests for the Host surface checks (ci#423).
//
// These tests exercise the compile-time surface conformance machinery: they
// prove that a minimal fake Host passes the check, that the check enumerates
// the expected Host method set, and that the merge/extension bucket index used
// by review bots stays in sync with the real Host interface.
package integrationinterface

import (
	"strings"
	"testing"
)

// testHostStub is a minimal Host used only to prove the surface-check
// machinery accepts a conforming implementation. It is a test helper, not a
// reference implementation.
type testHostStub struct{}

func (testHostStub) Version() int                    { return HostVersion }
func (testHostStub) Config() Config                  { return nil }
func (testHostStub) ConfigWrite() ConfigWriter      { return nil }
func (testHostStub) Logger() Logger                  { return nil }
func (testHostStub) HTTP() HTTP                      { return nil }
func (testHostStub) Storage() Storage                { return nil }
func (testHostStub) Chat() Chat                      { return nil }
func (testHostStub) Workspace() Workspace            { return nil }
func (testHostStub) Users() UsersService             { return nil }
func (testHostStub) Web() Web                        { return nil }
func (testHostStub) Tokens() Tokens                  { return nil }
func (testHostStub) Tools() Tools                    { return nil }

func TestCheckHostSurfaceAcceptsConformingHost(t *testing.T) {
	if err := checkHostSurface(testHostStub{}); err != nil {
		t.Fatalf("conforming Host failed the surface check: %v", err)
	}
}

func TestCheckHostSurfaceRejectsNilHost(t *testing.T) {
	if err := checkHostSurface(nil); err == nil {
		t.Fatal("nil Host passed the surface check; want error")
	}
}

// TestCheckHostSurfaceEnumeratedMethodSet pins the exact method set the host
// surface check covers, so a capability added to Host without a corresponding
// bucket breaks this test and forces the bucket index to be updated.
func TestCheckHostSurfaceEnumeratedMethodSet(t *testing.T) {
	wantKeys := []string{
		"config", "storage", "http", "logging", "security", "web", "tools", "versioning",
	}
	if len(hostSurfaceBuckets) != len(wantKeys) {
		t.Fatalf("hostSurfaceBuckets has %d buckets; want %d (add a bucket entry for every new Host capability)", len(hostSurfaceBuckets), len(wantKeys))
	}
	var got []string
	for bucket := range hostSurfaceBuckets {
		got = append(got, bucket)
	}
	for _, want := range wantKeys {
		found := false
		for _, g := range got {
			if g == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("bucket %q missing from hostSurfaceBuckets; add it", want)
		}
	}
}

// The surface check must never allow a missing method through: for every
// method name declared on the current Host interface, the check must be able
// to detect a host lacking that method. We verify per method by deleting it
// from hostSurfaceBuckets and confirming the check reports it as missing.
func TestCheckHostSurfaceDetectsMissingMethods(t *testing.T) {
	for _, methods := range hostSurfaceBuckets {
		for _, method := range methods {
			methods := methods // copy for the closure
			t.Run(method, func(t *testing.T) {
				saved := hostSurfaceBuckets
				hostSurfaceBuckets = cloneBuckets(saved)
				removeMethod(hostSurfaceBuckets, method)
				defer func() { hostSurfaceBuckets = saved }()

				err := checkHostSurface(testHostStub{})
				if err == nil {
					t.Fatalf("check passed despite %s being removed from the bucket index", method)
				}
				if !strings.Contains(err.Error(), method) {
					t.Fatalf("check errored on %s but message %q does not name the missing method", method, err)
				}
			})
		}
	}
}

func cloneBuckets(b HostSurfaceBuckets) HostSurfaceBuckets {
	out := make(HostSurfaceBuckets, len(b))
	for k, v := range b {
		methods := make([]string, len(v))
		copy(methods, v)
		out[k] = methods
	}
	return out
}

func removeMethod(b HostSurfaceBuckets, method string) {
	for bucket, methods := range b {
		for i, m := range methods {
			if m == method {
				b[bucket] = append(methods[:i], methods[i+1:]...)
				return
			}
		}
	}
}
