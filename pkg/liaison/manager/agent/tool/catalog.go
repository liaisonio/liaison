package tool

import (
	"strings"
	"sync"
)

type catalogEntry struct {
	descriptor             ToolDescriptor
	registrationGeneration uint64
	status                 ToolStatus
}

type catalogSnapshot struct {
	generation uint64
	entries    map[ToolID]catalogEntry
}

type Catalog struct {
	mu      sync.RWMutex
	current catalogSnapshot
}

func NewCatalog() *Catalog {
	return &Catalog{current: catalogSnapshot{entries: make(map[ToolID]catalogEntry)}}
}

func (catalog *Catalog) snapshot() catalogSnapshot {
	catalog.mu.RLock()
	defer catalog.mu.RUnlock()
	entries := make(map[ToolID]catalogEntry, len(catalog.current.entries))
	for id, entry := range catalog.current.entries {
		entry.descriptor = cloneDescriptor(entry.descriptor)
		entries[id] = entry
	}
	return catalogSnapshot{generation: catalog.current.generation, entries: entries}
}

func (catalog *Catalog) upsert(descriptor ToolDescriptor, registrationGeneration uint64, status ToolStatus) uint64 {
	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	entries := cloneCatalogEntries(catalog.current.entries)
	entries[descriptor.ID] = catalogEntry{
		descriptor:             cloneDescriptor(descriptor),
		registrationGeneration: registrationGeneration,
		status:                 status,
	}
	catalog.current = catalogSnapshot{generation: catalog.current.generation + 1, entries: entries}
	return catalog.current.generation
}

func (catalog *Catalog) setStatus(id ToolID, status ToolStatus) (uint64, bool) {
	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	entry, ok := catalog.current.entries[id]
	if !ok {
		return catalog.current.generation, false
	}
	entries := cloneCatalogEntries(catalog.current.entries)
	entry.status = status
	entries[id] = entry
	catalog.current = catalogSnapshot{generation: catalog.current.generation + 1, entries: entries}
	return catalog.current.generation, true
}

func (catalog *Catalog) remove(id ToolID) (uint64, bool) {
	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	if _, ok := catalog.current.entries[id]; !ok {
		return catalog.current.generation, false
	}
	entries := cloneCatalogEntries(catalog.current.entries)
	delete(entries, id)
	catalog.current = catalogSnapshot{generation: catalog.current.generation + 1, entries: entries}
	return catalog.current.generation, true
}

func (catalog *Catalog) byModelName(namespace, name string) []catalogEntry {
	snapshot := catalog.snapshot()
	entries := make([]catalogEntry, 0, 1)
	for id, entry := range snapshot.entries {
		if id.Namespace == namespace && id.Name == name {
			entries = append(entries, entry)
		}
	}
	return entries
}

func cloneCatalogEntries(source map[ToolID]catalogEntry) map[ToolID]catalogEntry {
	entries := make(map[ToolID]catalogEntry, len(source))
	for id, entry := range source {
		entries[id] = entry
	}
	return entries
}

func descriptorMatchScore(descriptor ToolDescriptor, query string) int {
	query = strings.ToLower(strings.TrimSpace(query))
	switch query {
	case "list all available tools", "list tools", "all tools", "available tools", "全部工具", "可用工具", "列出所有工具":
		query = ""
	}
	if query == "" {
		return 1
	}
	queryParts := strings.Fields(query)
	haystacks := []struct {
		value  string
		weight int
	}{
		{strings.ToLower(descriptor.ID.ModelName()), 8},
		{strings.ToLower(descriptor.DisplayName), 6},
		{strings.ToLower(strings.Join(descriptor.Tags, " ")), 4},
		{strings.ToLower(descriptor.WhenToUse), 3},
		{strings.ToLower(descriptor.Description), 2},
	}
	score := 0
	for _, part := range queryParts {
		matched := false
		for _, haystack := range haystacks {
			if strings.Contains(haystack.value, part) {
				score += haystack.weight
				matched = true
			}
		}
		if !matched {
			return 0
		}
	}
	return score
}
