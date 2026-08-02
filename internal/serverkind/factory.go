// For maps a server type name to its Resolver implementation.
package serverkind

import "fmt"

// Kind describes a supported server flavor for menus and help text, so the
// list of choices lives next to the implementations rather than being
// hand-copied into every command that offers them.
type Kind struct {
	Name        string
	Summary     string
	Description string
}

// Kinds lists the supported server flavors in the order they should be offered.
func Kinds() []Kind {
	return []Kind{
		{
			Name:        "vanilla",
			Summary:     "Official Mojang server",
			Description: "Unmodified gameplay, straight from Mojang. Slower with many players.",
		},
		{
			Name:        "paper",
			Summary:     "PaperMC — faster, supports plugins",
			Description: "Drop-in vanilla replacement with big performance gains and plugin support.",
		},
	}
}

// KindNames returns just the flavor names, for flag validation and completion.
func KindNames() []string {
	kinds := Kinds()
	names := make([]string, len(kinds))
	for i, k := range kinds {
		names[i] = k.Name
	}
	return names
}

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
