package realtime

import (
	"sync"
)

// SubscriptionIndex provides efficient lookup of subscriptions by collection.
type SubscriptionIndex struct {
	byCollection map[string]map[string]*Subscription
	mu           sync.RWMutex
}

// NewSubscriptionIndex creates a new subscription index.
func NewSubscriptionIndex() *SubscriptionIndex {
	return &SubscriptionIndex{
		byCollection: make(map[string]map[string]*Subscription),
	}
}

// Add indexes a subscription.
func (idx *SubscriptionIndex) Add(sub *Subscription) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.byCollection[sub.Collection] == nil {
		idx.byCollection[sub.Collection] = make(map[string]*Subscription)
	}
	idx.byCollection[sub.Collection][sub.ID] = sub
}

// Remove removes a subscription from the index.
func (idx *SubscriptionIndex) Remove(sub *Subscription) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if subs, ok := idx.byCollection[sub.Collection]; ok {
		delete(subs, sub.ID)
		if len(subs) == 0 {
			delete(idx.byCollection, sub.Collection)
		}
	}
}

// GetCandidates returns all subscriptions for a collection.
func (idx *SubscriptionIndex) GetCandidates(collection string) []*Subscription {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	subs, ok := idx.byCollection[collection]
	if !ok {
		return nil
	}

	result := make([]*Subscription, 0, len(subs))
	for _, sub := range subs {
		result = append(result, sub)
	}
	return result
}

// Count returns the total number of indexed subscriptions.
func (idx *SubscriptionIndex) Count() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	count := 0
	for _, subs := range idx.byCollection {
		count += len(subs)
	}
	return count
}

// CollectionCount returns the number of subscriptions for a collection.
func (idx *SubscriptionIndex) CollectionCount(collection string) int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if subs, ok := idx.byCollection[collection]; ok {
		return len(subs)
	}
	return 0
}
