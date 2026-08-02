// cache memoizes upstream metadata fetches for the lifetime of the process.
//
// Creating a server used to fetch Mojang's version manifest four times over:
// once for the jar URL and once for the Java requirement, each of which walked
// the manifest and then the per-version document. The interactive create
// wizard makes that worse, since it also lists versions up front. Entries are
// keyed by URL so tests that repoint the API endpoints are unaffected.
package serverkind

import "sync"

var fetchCache sync.Map // url string -> cached value

// fetchOnce returns the memoized result for url, calling load to produce it
// the first time. Failures are not cached: a network blip during the wizard
// should not poison the rest of the session. Two concurrent misses may both
// call load, which is harmless for idempotent GETs.
func fetchOnce[T any](url string, load func() (T, error)) (T, error) {
	if v, ok := fetchCache.Load(url); ok {
		return v.(T), nil
	}
	v, err := load()
	if err != nil {
		var zero T
		return zero, err
	}
	fetchCache.Store(url, v)
	return v, nil
}
