// SPDX-License-Identifier: MIT
package marketplace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type LocalStore struct {
	dataDir string
	mu      sync.RWMutex
}

func NewLocalStore(dataDir string) *LocalStore {
	return &LocalStore{dataDir: dataDir}
}

func (s *LocalStore) List(ctx context.Context, filter Filter) ([]Listing, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	var listings []Listing
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		listing, err := s.readFileLocked(id)
		if err != nil {
			continue
		}
		if filter.Type != nil && listing.Type != *filter.Type {
			continue
		}
		if filter.MinRating != nil && listing.Rating < *filter.MinRating {
			continue
		}
		if len(filter.Tags) > 0 && !hasAllTags(listing.Tags, filter.Tags) {
			continue
		}
		listings = append(listings, *listing)
	}

	sortListings(listings, filter.SortBy, filter.SortOrder)
	return listings, nil
}

func (s *LocalStore) Search(ctx context.Context, query string) ([]Listing, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	q := strings.ToLower(query)
	var results []Listing
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		listing, err := s.readFileLocked(id)
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(listing.Name), q) ||
			strings.Contains(strings.ToLower(listing.Description), q) ||
			tagContains(listing.Tags, q) {
			results = append(results, *listing)
		}
	}
	return results, nil
}

func (s *LocalStore) Get(ctx context.Context, id string) (*Listing, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readFileLocked(id)
}

func (s *LocalStore) Register(ctx context.Context, listing Listing) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	listing.UpdatedAt = time.Now()
	return s.writeFileLocked(listing)
}

func (s *LocalStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.dataDir, id+".json")
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove: %w", err)
	}
	return nil
}

func (s *LocalStore) readFileLocked(id string) (*Listing, error) {
	path := filepath.Join(s.dataDir, id+".json")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	var listing Listing
	if err := json.NewDecoder(f).Decode(&listing); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &listing, nil
}

func (s *LocalStore) writeFileLocked(listing Listing) error {
	if listing.ID == "" {
		return fmt.Errorf("listing ID is required")
	}
	if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	path := filepath.Join(s.dataDir, listing.ID+".json")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(listing); err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	return nil
}

func hasAllTags(listingTags, filterTags []string) bool {
	for _, ft := range filterTags {
		found := false
		for _, lt := range listingTags {
			if strings.EqualFold(lt, ft) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func tagContains(tags []string, q string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}

func sortListings(listings []Listing, sortBy, sortOrder string) {
	if sortBy == "" {
		return
	}
	asc := strings.EqualFold(sortOrder, "asc")
	sort.Slice(listings, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "name":
			less = listings[i].Name < listings[j].Name
		case "downloads":
			less = listings[i].Downloads < listings[j].Downloads
		case "rating":
			less = listings[i].Rating < listings[j].Rating
		case "updated_at":
			less = listings[i].UpdatedAt.Before(listings[j].UpdatedAt)
		default:
			less = listings[i].Name < listings[j].Name
		}
		if asc {
			return less
		}
		return !less
	})
}
