// For maps a server type name to its Resolver implementation.
package serverkind

import "fmt"

// For returns the Resolver for a server type name ("vanilla" or "paper").
func For(kind string) (Resolver, error) {
	switch kind {
	case "vanilla":
		return Vanilla{}, nil
	case "paper":
		return Paper{}, nil
	default:
		return nil, fmt.Errorf("unknown server type %q (want vanilla or paper)", kind)
	}
}
