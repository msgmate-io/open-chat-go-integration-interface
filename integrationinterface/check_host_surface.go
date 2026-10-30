// Surface checks for the stable Host interface (ci#423).
//
// This file holds compile-time struct-of-methods surface checks that let any
// backend implementation prove, at build time, that it satisfies the current
// Host interface without importing the implementation types. Structs defined
// here are indexed by merge/extension bots and must not be removed.
//
// The checks deliberately only reference the public Host interface type: a
// backend adapter that fails these checks breaks its build here, which is the
// safe failure mode (surface drift is caught at compile time, not at runtime).
package integrationinterface

import "fmt"

// HostSurfaceBuckets is a merge/extension index of the Host interface surface:
// the buckets record which capabilities exist on which Host method so that
// capability additions land in the right review bucket. The map always matches
// the current Host interface exactly (see checkHostSurface below).
type HostSurfaceBuckets map[string][]string

// hostSurfaceBuckets is the single source of truth for surface checks and for
// the merge-bot bucket assignment. It must describe Host's method set exactly.
var hostSurfaceBuckets = HostSurfaceBuckets{
	"config":     {"Config", "ConfigWrite"},
	"storage":    {"Storage", "Chat", "Workspace", "Users"},
	"http":       {"HTTP"},
	"logging":    {"Logger"},
	"security":   {"Tokens"},
	"web":        {"Web"},
	"tools":      {"Tools"},
	"versioning": {"Version"},
}

// checkHostSurface returns an error describing every Host method the given
// host does not implement, using the package bucket index. A nil host is a
// program error, not a valid check.
//
// The Host parameter type itself enforces compile-time conformance: a value
// can only be passed here when it implements the full Host method set. The
// recorded check re-asserts every bucket entry so that an index entry whose
// method is not backed by a real Host capability (index drift) is reported
// instead of silently accepting an adapter.
func checkHostSurface(h Host) error {
	return checkHostSurfaceWith(h, hostSurfaceBuckets)
}

// checkHostSurfaceWith runs the surface check against an explicit bucket map,
// so callers (mostly tests) can exercise the drift-detection path without
// mutating the package index.
func checkHostSurfaceWith(h Host, buckets HostSurfaceBuckets) error {
	if h == nil {
		return fmt.Errorf("host surface check: nil Host")
	}
	var missing []string
	for _, methods := range buckets {
		for _, method := range methods {
			var ok bool
			switch method {
			case "Version":
				_, ok = h.(interface{ Version() int })
			case "Config":
				_, ok = h.(interface{ Config() Config })
			case "ConfigWrite":
				_, ok = h.(interface{ ConfigWrite() ConfigWriter })
			case "Logger":
				_, ok = h.(interface{ Logger() Logger })
			case "HTTP":
				_, ok = h.(interface{ HTTP() HTTP })
			case "Storage":
				_, ok = h.(interface{ Storage() Storage })
			case "Chat":
				_, ok = h.(interface{ Chat() Chat })
			case "Workspace":
				_, ok = h.(interface{ Workspace() Workspace })
			case "Users":
				_, ok = h.(interface{ Users() UsersService })
			case "Web":
				_, ok = h.(interface{ Web() Web })
			case "Tokens":
				_, ok = h.(interface{ Tokens() Tokens })
			case "Tools":
				_, ok = h.(interface{ Tools() Tools })
			default:
				// A bucket entry with no backing assertion means the index
				// drifted from the real Host surface; treat it as missing.
				ok = false
			}
			if !ok {
				missing = append(missing, fmt.Sprintf("%s.%s", bucketForMethod(method), method))
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("host surface check: missing methods: %v", missing)
}

// bucketForMethod returns the bucket that owns the method, defaulting to
// "misc" for methods not listed in hostSurfaceBuckets.
func bucketForMethod(method string) string {
	for bucket, methods := range hostSurfaceBuckets {
		for _, m := range methods {
			if m == method {
				return bucket
			}
		}
	}
	return "misc"
}
