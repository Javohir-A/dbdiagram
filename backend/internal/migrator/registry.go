package migrator

import "fmt"

// Set is one migration folder and the Postgres schema it owns.
type Set struct {
	Name   string // migrations/<Name>
	Schema string
}

// sets — ORDER MATTERS: shared first (helper functions), then the schema that
// uses them.
var sets = []Set{
	{Name: "shared", Schema: "shared"},
	{Name: "dbdiagram", Schema: "dbdiagram"},
}

func Targets(name string) ([]Set, error) {
	if name == "" {
		return sets, nil
	}

	for _, s := range sets {
		if s.Name == name {
			return []Set{s}, nil
		}
	}

	return nil, fmt.Errorf("set %q is not registered in registry.go", name)
}
