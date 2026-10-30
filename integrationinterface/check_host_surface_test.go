// Tests for the Host surface checks (ci#423).
//
// These tests exercise the compile-time surface conformance machinery: they
// prove that a minimal fake Host passes the check, that the check rejects a
// nil host, that the merge/extension bucket index used by review bots stays in
// exact sync with the real Host interface method set, and that the check
// reports methods a Host does not implement.
package integrationinterface

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// testHostStub is a minimal Host used only to prove the surface-check
// machinery accepts a conforming implementation. It is a test helper, not a
// reference implementation.
type testHostStub struct{}

func (testHostStub) Version() int              { return HostVersion }
func (testHostStub) Config() Config            { return nil }
func (testHostStub) ConfigWrite() ConfigWriter { return nil }
func (testHostStub) Logger() Logger            { return nil }
func (testHostStub) HTTP() HTTP                { return nil }
func (testHostStub) Storage() Storage          { return nil }
func (testHostStub) Chat() Chat                { return nil }
func (testHostStub) Workspace() Workspace      { return nil }
func (testHostStub) Users() UsersService       { return nil }
func (testHostStub) Web() Web                  { return nil }
func (testHostStub) Tokens() Tokens            { return nil }
func (testHostStub) Tools() Tools              { return nil }

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

// hostMethodSet returns the sorted method names declared on the real Host
// interface type, using reflection.
func hostMethodSet() []string {
	typ := reflect.TypeOf((*Host)(nil)).Elem()
	names := make([]string, 0, typ.NumMethod())
	for i := 0; i < typ.NumMethod(); i++ {
		names = append(names, typ.Method(i).Name)
	}
	sort.Strings(names)
	return names
}

// bucketMethodSet returns the sorted method names recorded in the bucket
// index, i.e. the union of every capability bucket.
func bucketMethodSet() []string {
	set := map[string]bool{}
	for _, methods := range hostSurfaceBuckets {
		for _, m := range methods {
			set[m] = true
		}
	}
	names := make([]string, 0, len(set))
	for m := range set {
		names = append(names, m)
	}
	sort.Strings(names)
	return names
}

// TestHostSurfaceBucketsMatchHostInterface pins the bucket index to the real
// Host interface: adding a capability to Host without a bucket entry, or
// dropping a method from the index, breaks this test. This keeps the
// merge/extension bucket assignment in sync with the shipped contract.
func TestHostSurfaceBucketsMatchHostInterface(t *testing.T) {
	hostMethods := hostMethodSet()
	bucketMethods := bucketMethodSet()
	if !reflect.DeepEqual(hostMethods, bucketMethods) {
		t.Fatalf("bucket index %v does not match Host interface method set %v; update hostSurfaceBuckets", bucketMethods, hostMethods)
	}
}

// TestCheckHostSurfaceReportsUnimplementedMethod proves the drift-detection
// path: a bucket entry whose method is not backed by a real Host capability
// must fail the check, and the error must name that entry. We run the explicit
// variant with a synthetic capability injected into a copy of the index so the
// package global stays untouched.
func TestCheckHostSurfaceReportsUnimplementedMethod(t *testing.T) {
	drifted := cloneBuckets(hostSurfaceBuckets)
	for bucket := range drifted {
		drifted[bucket] = append(drifted[bucket], "SyntheticCapability")
		break
	}

	err := checkHostSurfaceWith(testHostStub{}, drifted)
	if err == nil {
		t.Fatal("check passed despite a recorded capability not backed by a real Host method")
	}
	if !strings.Contains(err.Error(), "SyntheticCapability") {
		t.Fatalf("check error %q does not name the drifted capability", err)
	}
}

// TestBucketForMethodCoversHostMethods verifies every Host method is owned by
// exactly one bucket (no "misc" leftovers).
func TestBucketForMethodCoversHostMethods(t *testing.T) {
	for _, method := range hostMethodSet() {
		if bucket := bucketForMethod(method); bucket == "misc" {
			t.Errorf("Host method %s is not owned by any capability bucket", method)
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
