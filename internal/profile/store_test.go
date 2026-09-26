package profile

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"waveguide/internal/physics"
)

func sample(name string) Profile {
	return Profile{
		Name:         name,
		CrossSection: physics.CrossSection{A: 22.86e-3, B: 10.16e-3, EpsilonR: 1},
	}
}

func TestAddGetListDelete(t *testing.T) {
	s := NewStore()
	if err := s.Add(sample("WR-90")); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := s.Add(sample("WR-62")); err != nil {
		t.Fatalf("add: %v", err)
	}

	p, err := s.Get("WR-90")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if p.A != 22.86e-3 {
		t.Fatalf("unexpected broad dimension %v", p.A)
	}

	list := s.List()
	if len(list) != 2 || list[0].Name != "WR-62" || list[1].Name != "WR-90" {
		t.Fatalf("list not sorted or incomplete: %+v", list)
	}

	if err := s.Delete("WR-90"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get("WR-90"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after delete: want ErrNotFound, got %v", err)
	}
}

func TestDuplicateNameConflict(t *testing.T) {
	s := NewStore()
	if err := s.Add(sample("WR-90")); err != nil {
		t.Fatalf("add: %v", err)
	}
	dup := sample("WR-90")
	dup.A = 1.0 // different content, same name: must NOT overwrite
	if err := s.Add(dup); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate add: want ErrConflict, got %v", err)
	}
	p, _ := s.Get("WR-90")
	if p.A != 22.86e-3 {
		t.Fatalf("duplicate add overwrote the record: a=%v", p.A)
	}
}

func TestDeleteMissingNotFound(t *testing.T) {
	s := NewStore()
	if err := s.Delete("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete missing: want ErrNotFound, got %v", err)
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := NewStore()
	SeedBuiltin(s)
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("WG-%d", i)
			_ = s.Add(sample(name))
			_, _ = s.Get(name)
			_ = s.List()
			_ = s.Delete(name)
		}(i)
	}
	wg.Wait()
	// Only the seeded builtin profile may remain.
	if got := s.List(); len(got) != len(BuiltinProfiles()) {
		t.Fatalf("expected only builtin profiles left, got %+v", got)
	}
}
