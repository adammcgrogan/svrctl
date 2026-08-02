// registry_helpers_test covers the lookup errors users actually hit.
package cli

import (
	"strings"
	"testing"

	"github.com/adammcgrogan/svrctl/internal/registry"
)

func regWith(names ...string) *registry.Registry {
	reg := &registry.Registry{Servers: map[string]registry.Server{}}
	for _, n := range names {
		reg.Put(n, registry.Server{Type: "paper", Version: "1.21.1"})
	}
	return reg
}

func TestUnknownServerSuggestsTheLikelyTypo(t *testing.T) {
	err := unknownServerError(regWith("survival", "creative"), "surviva")

	if !strings.Contains(err.Error(), `did you mean "survival"`) {
		t.Errorf("got %q, want a suggestion", err)
	}
}

func TestUnknownServerDoesNotGuessWildly(t *testing.T) {
	err := unknownServerError(regWith("survival"), "minigames")

	if strings.Contains(err.Error(), "did you mean") {
		t.Errorf("suggested an unrelated name: %q", err)
	}
	if !strings.Contains(err.Error(), "svrctl list") {
		t.Errorf("got %q, want it to point at the listing", err)
	}
}

func TestUnknownServerWithEmptyRegistryPointsAtCreate(t *testing.T) {
	err := unknownServerError(regWith(), "survival")

	if !strings.Contains(err.Error(), "svrctl create") {
		t.Errorf("got %q, want it to suggest creating one", err)
	}
}

func TestEditDistance(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"survival", "survival", 0},
		{"surviva", "survival", 1},
		{"survivl", "survival", 1},
		{"", "abc", 3},
		{"abc", "", 3},
		{"kitten", "sitting", 3},
	}
	for _, c := range cases {
		if got := editDistance(c.a, c.b); got != c.want {
			t.Errorf("editDistance(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestServerViewPortFallsBackToMinecraftDefault(t *testing.T) {
	unset := serverView{Server: registry.Server{}}
	if got := unset.port(); got != 25565 {
		t.Errorf("got %d, want Minecraft's default of 25565", got)
	}

	set := serverView{Server: registry.Server{Port: 25570}}
	if got := set.port(); got != 25570 {
		t.Errorf("got %d, want the configured 25570", got)
	}
}

func TestJoinPhrasing(t *testing.T) {
	cases := []struct {
		items []string
		or    string
		and   string
	}{
		{[]string{"vanilla"}, "vanilla", "vanilla"},
		{[]string{"vanilla", "paper"}, "vanilla or paper", "vanilla and paper"},
		{[]string{"a", "b", "c"}, "a, b or c", "a, b and c"},
	}
	for _, c := range cases {
		if got := joinOr(c.items); got != c.or {
			t.Errorf("joinOr(%v) = %q, want %q", c.items, got, c.or)
		}
		if got := joinAnd(c.items); got != c.and {
			t.Errorf("joinAnd(%v) = %q, want %q", c.items, got, c.and)
		}
	}
}
