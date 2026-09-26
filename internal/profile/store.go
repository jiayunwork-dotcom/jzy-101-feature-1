// Package profile implements the in-memory, concurrency-safe registry of
// named waveguide cross-section profiles.
package profile

import (
	"errors"
	"sort"
	"sync"

	"waveguide/internal/physics"
)

// Store errors mapped to HTTP statuses by the API layer.
var (
	ErrNotFound = errors.New("profile not found")
	ErrConflict = errors.New("a profile with this name already exists")
)

// Profile is a named, reusable waveguide cross-section specification.
type Profile struct {
	Name string `json:"name"`
	physics.CrossSection
}

// Store keeps profiles in a map guarded by a RWMutex, so concurrent
// engineers can register, list, delete and query profiles safely.
type Store struct {
	mu       sync.RWMutex
	profiles map[string]Profile
}

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{profiles: make(map[string]Profile)}
}

// Add registers a new profile. It never overwrites an existing record:
// a duplicate name yields ErrConflict.
func (s *Store) Add(p Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.profiles[p.Name]; exists {
		return ErrConflict
	}
	s.profiles[p.Name] = p
	return nil
}

// Get returns the named profile, or ErrNotFound.
func (s *Store) Get(name string) (Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.profiles[name]
	if !ok {
		return Profile{}, ErrNotFound
	}
	return p, nil
}

// Delete removes the named profile. Deleting a missing profile is an
// explicit ErrNotFound, never a silent success.
func (s *Store) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.profiles[name]; !exists {
		return ErrNotFound
	}
	delete(s.profiles, name)
	return nil
}

// List returns all profiles sorted by name. The result is a copy;
// callers may not mutate the store through it.
func (s *Store) List() []Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Profile, 0, len(s.profiles))
	for _, p := range s.profiles {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// BuiltinProfiles returns the sample profiles seeded at startup. WR-90
// is the standard X-band rectangular waveguide (a = 22.86 mm,
// b = 10.16 mm, air-filled): its dominant TE10 mode cuts off at
// ~6.557 GHz and the next mode (TE20) at ~13.114 GHz, so the whole
// X-band (8.2-12.4 GHz) sits squarely in the single-mode range.
func BuiltinProfiles() []Profile {
	return []Profile{
		{
			Name: "WR-90",
			CrossSection: physics.CrossSection{
				A:        22.86e-3,
				B:        10.16e-3,
				EpsilonR: 1.0,
			},
		},
	}
}

// SeedBuiltin registers the builtin sample profiles, ignoring conflicts
// so it is safe to call on a pre-populated store.
func SeedBuiltin(s *Store) {
	for _, p := range BuiltinProfiles() {
		_ = s.Add(p)
	}
}
