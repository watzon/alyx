package requestlog

import (
	"testing"
	"time"
)

func TestStore_Add(t *testing.T) {
	store := NewStore(3)

	store.Add(Entry{ID: "1", Method: "GET", Path: "/a"})
	store.Add(Entry{ID: "2", Method: "POST", Path: "/b"})

	if store.Count() != 2 {
		t.Errorf("Count() = %d, want 2", store.Count())
	}
}

func TestStore_RingBuffer(t *testing.T) {
	store := NewStore(3)

	store.Add(Entry{ID: "1", Method: "GET", Path: "/a"})
	store.Add(Entry{ID: "2", Method: "POST", Path: "/b"})
	store.Add(Entry{ID: "3", Method: "PUT", Path: "/c"})
	store.Add(Entry{ID: "4", Method: "DELETE", Path: "/d"})

	if store.Count() != 3 {
		t.Errorf("Count() = %d, want 3 (capacity)", store.Count())
	}

	result := store.List(FilterOptions{Limit: 10})
	if len(result.Entries) != 3 {
		t.Errorf("List returned %d entries, want 3", len(result.Entries))
	}

	// Newest first
	if result.Entries[0].ID != "4" {
		t.Errorf("First entry ID = %s, want 4 (newest)", result.Entries[0].ID)
	}
	if result.Entries[2].ID != "2" {
		t.Errorf("Last entry ID = %s, want 2 (oldest remaining)", result.Entries[2].ID)
	}
}

func TestStore_FilterByMethod(t *testing.T) {
	store := NewStore(10)

	store.Add(Entry{ID: "1", Method: "GET", Path: "/a"})
	store.Add(Entry{ID: "2", Method: "POST", Path: "/b"})
	store.Add(Entry{ID: "3", Method: "GET", Path: "/c"})

	result := store.List(FilterOptions{Method: "GET"})
	if result.Total != 2 {
		t.Errorf("Total = %d, want 2", result.Total)
	}
}

func TestStore_FilterByStatus(t *testing.T) {
	store := NewStore(10)

	store.Add(Entry{ID: "1", Status: 200})
	store.Add(Entry{ID: "2", Status: 404})
	store.Add(Entry{ID: "3", Status: 500})
	store.Add(Entry{ID: "4", Status: 201})

	t.Run("exact status", func(t *testing.T) {
		result := store.List(FilterOptions{Status: 200})
		if result.Total != 1 {
			t.Errorf("Total = %d, want 1", result.Total)
		}
	})

	t.Run("min status", func(t *testing.T) {
		result := store.List(FilterOptions{MinStatus: 400})
		if result.Total != 2 {
			t.Errorf("Total = %d, want 2", result.Total)
		}
	})

	t.Run("max status", func(t *testing.T) {
		result := store.List(FilterOptions{MaxStatus: 299})
		if result.Total != 2 {
			t.Errorf("Total = %d, want 2", result.Total)
		}
	})

	t.Run("status range", func(t *testing.T) {
		result := store.List(FilterOptions{MinStatus: 200, MaxStatus: 299})
		if result.Total != 2 {
			t.Errorf("Total = %d, want 2", result.Total)
		}
	})
}

func TestStore_FilterByTime(t *testing.T) {
	store := NewStore(10)

	now := time.Now()
	store.Add(Entry{ID: "1", Timestamp: now.Add(-2 * time.Hour)})
	store.Add(Entry{ID: "2", Timestamp: now.Add(-1 * time.Hour)})
	store.Add(Entry{ID: "3", Timestamp: now})

	result := store.List(FilterOptions{Since: now.Add(-90 * time.Minute)})
	if result.Total != 2 {
		t.Errorf("Total = %d, want 2", result.Total)
	}
}

func TestStore_FilterByExcludePathPrefix(t *testing.T) {
	store := NewStore(10)

	store.Add(Entry{ID: "1", Path: "/api/admin/logs"})
	store.Add(Entry{ID: "2", Path: "/api/admin/users"})
	store.Add(Entry{ID: "3", Path: "/api/collections/posts"})
	store.Add(Entry{ID: "4", Path: "/api/functions"})

	t.Run("exclude admin paths", func(t *testing.T) {
		result := store.List(FilterOptions{ExcludePathPrefix: "/api/admin"})
		if result.Total != 2 {
			t.Errorf("Total = %d, want 2", result.Total)
		}
	})

	t.Run("no exclusion", func(t *testing.T) {
		result := store.List(FilterOptions{})
		if result.Total != 4 {
			t.Errorf("Total = %d, want 4", result.Total)
		}
	})

	t.Run("exclude non-matching prefix", func(t *testing.T) {
		result := store.List(FilterOptions{ExcludePathPrefix: "/api/other"})
		if result.Total != 4 {
			t.Errorf("Total = %d, want 4", result.Total)
		}
	})
}

func TestStore_Pagination(t *testing.T) {
	store := NewStore(100)

	for i := 0; i < 25; i++ {
		store.Add(Entry{ID: string(rune('a' + i))})
	}

	t.Run("first page", func(t *testing.T) {
		result := store.List(FilterOptions{Limit: 10, Offset: 0})
		if len(result.Entries) != 10 {
			t.Errorf("Entries = %d, want 10", len(result.Entries))
		}
		if result.Total != 25 {
			t.Errorf("Total = %d, want 25", result.Total)
		}
	})

	t.Run("second page", func(t *testing.T) {
		result := store.List(FilterOptions{Limit: 10, Offset: 10})
		if len(result.Entries) != 10 {
			t.Errorf("Entries = %d, want 10", len(result.Entries))
		}
	})

	t.Run("last page", func(t *testing.T) {
		result := store.List(FilterOptions{Limit: 10, Offset: 20})
		if len(result.Entries) != 5 {
			t.Errorf("Entries = %d, want 5", len(result.Entries))
		}
	})

	t.Run("beyond end", func(t *testing.T) {
		result := store.List(FilterOptions{Limit: 10, Offset: 100})
		if len(result.Entries) != 0 {
			t.Errorf("Entries = %d, want 0", len(result.Entries))
		}
	})
}

func TestStore_Clear(t *testing.T) {
	store := NewStore(10)

	store.Add(Entry{ID: "1"})
	store.Add(Entry{ID: "2"})
	store.Clear()

	if store.Count() != 0 {
		t.Errorf("Count() = %d, want 0", store.Count())
	}
}

func TestStore_Stats(t *testing.T) {
	store := NewStore(100)

	store.Add(Entry{ID: "1"})
	store.Add(Entry{ID: "2"})

	stats := store.Stats()
	if stats.Capacity != 100 {
		t.Errorf("Capacity = %d, want 100", stats.Capacity)
	}
	if stats.Count != 2 {
		t.Errorf("Count = %d, want 2", stats.Count)
	}
}

func TestStore_DefaultCapacity(t *testing.T) {
	store := NewStore(0)
	if store.capacity != 1000 {
		t.Errorf("capacity = %d, want 1000 (default)", store.capacity)
	}

	store = NewStore(-5)
	if store.capacity != 1000 {
		t.Errorf("capacity = %d, want 1000 (default)", store.capacity)
	}
}

func TestStore_LimitCapping(t *testing.T) {
	store := NewStore(10)

	for i := 0; i < 10; i++ {
		store.Add(Entry{ID: string(rune('a' + i))})
	}

	t.Run("default limit", func(t *testing.T) {
		result := store.List(FilterOptions{})
		if result.Limit != 100 {
			t.Errorf("Limit = %d, want 100 (default)", result.Limit)
		}
	})

	t.Run("cap at 1000", func(t *testing.T) {
		result := store.List(FilterOptions{Limit: 5000})
		if result.Limit != 1000 {
			t.Errorf("Limit = %d, want 1000 (max)", result.Limit)
		}
	})
}
