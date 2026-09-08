package tool

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

type DisclosureEngine struct {
	catalog    *Catalog
	visibility VisibilityFilter
}

func NewDisclosureEngine(catalog *Catalog, visibility VisibilityFilter) *DisclosureEngine {
	if visibility == nil {
		visibility = AllowAllVisibility{}
	}
	return &DisclosureEngine{catalog: catalog, visibility: visibility}
}

func (engine *DisclosureEngine) BuildSnapshot(ctx context.Context, request DisclosureRequest) (ToolSetSnapshot, error) {
	budget := normalizeBudget(request.Budget)
	catalog := engine.catalog.snapshot()
	primary := findPrimaryAttachment(request.Attachments, request.Primary)
	promoted := toolIDSet(request.Promoted)
	recent := toolIDSet(request.RecentlyUsed)

	candidates := make([]rankedTool, 0, len(catalog.entries))
	for _, entry := range catalog.entries {
		if entry.status != ToolStatusActive || !descriptorMatchesSession(entry.descriptor, request.SessionKind) {
			continue
		}
		attachment := matchingAttachment(request.Attachments, primary, entry.descriptor)
		if !descriptorMatchesAttachment(entry.descriptor, attachment) {
			continue
		}
		allowed, err := engine.visibility.AllowTool(ctx, request.Principal, attachment, entry.descriptor)
		if err != nil {
			return ToolSetSnapshot{}, fmt.Errorf("evaluate tool visibility for %s: %w", entry.descriptor.ID.String(), err)
		}
		if !allowed {
			continue
		}
		_, isPromoted := promoted[entry.descriptor.ID]
		_, wasRecent := recent[entry.descriptor.ID]
		// Deferred tools are searchable by summary but their full JSON schema is
		// only disclosed after an explicit promotion (tool_describe).
		if entry.descriptor.Disclosure == DisclosureDeferred && !isPromoted {
			continue
		}
		score := disclosureScore(entry.descriptor, request.Query, isPromoted, wasRecent, attachment != nil)
		candidates = append(candidates, rankedTool{entry: entry, score: score, promoted: isPromoted})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].promoted != candidates[j].promoted {
			return candidates[i].promoted
		}
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].entry.descriptor.Risk != candidates[j].entry.descriptor.Risk {
			return candidates[i].entry.descriptor.Risk < candidates[j].entry.descriptor.Risk
		}
		return candidates[i].entry.descriptor.ID.String() < candidates[j].entry.descriptor.ID.String()
	})

	tools := make([]ExposedTool, 0, min(len(candidates), budget.MaxSchemas))
	tokens := 0
	alwaysVisible := 0
	deferredCandidates := 0
	for _, candidate := range candidates {
		descriptor := candidate.entry.descriptor
		if descriptor.Disclosure == DisclosureAlways {
			if alwaysVisible >= budget.MaxAlwaysVisible {
				continue
			}
			alwaysVisible++
		} else if descriptor.Disclosure == DisclosureDeferred && !candidate.promoted {
			if deferredCandidates >= budget.MaxCandidates {
				continue
			}
			deferredCandidates++
		}
		if len(tools) >= budget.MaxSchemas {
			break
		}
		estimated := estimateDescriptorTokens(descriptor)
		if tokens+estimated > budget.MaxTokens {
			continue
		}
		tools = append(tools, ExposedTool{
			Descriptor:             cloneDescriptor(descriptor),
			RegistrationGeneration: candidate.entry.registrationGeneration,
			EstimatedTokens:        estimated,
		})
		tokens += estimated
	}

	snapshotID, err := newSnapshotID()
	if err != nil {
		return ToolSetSnapshot{}, fmt.Errorf("create tool snapshot ID: %w", err)
	}
	attachmentGenerations := make(map[string]uint64, len(request.Attachments))
	for _, attachment := range request.Attachments {
		attachmentGenerations[attachment.ID] = attachment.Generation
	}
	return ToolSetSnapshot{
		ID:                   snapshotID,
		CatalogGeneration:    catalog.generation,
		PolicyRevision:       request.PolicyRevision,
		AttachmentGeneration: attachmentGenerations,
		Tools:                tools,
		CreatedAt:            time.Now().UTC(),
	}, nil
}

func (engine *DisclosureEngine) Search(ctx context.Context, request DisclosureRequest, query string) ([]ToolSummary, error) {
	budget := normalizeBudget(request.Budget)
	catalog := engine.catalog.snapshot()
	primary := findPrimaryAttachment(request.Attachments, request.Primary)
	type scoredSummary struct {
		score   int
		summary ToolSummary
	}
	results := make([]scoredSummary, 0, budget.MaxCandidates)
	for _, entry := range catalog.entries {
		if entry.status != ToolStatusActive || !descriptorMatchesSession(entry.descriptor, request.SessionKind) {
			continue
		}
		attachment := matchingAttachment(request.Attachments, primary, entry.descriptor)
		if !descriptorMatchesAttachment(entry.descriptor, attachment) {
			continue
		}
		allowed, err := engine.visibility.AllowTool(ctx, request.Principal, attachment, entry.descriptor)
		if err != nil {
			return nil, fmt.Errorf("evaluate tool visibility for %s: %w", entry.descriptor.ID.String(), err)
		}
		if !allowed {
			continue
		}
		score := descriptorMatchScore(entry.descriptor, strings.ToLower(strings.TrimSpace(query)))
		if strings.TrimSpace(query) != "" && score == 0 {
			continue
		}
		results = append(results, scoredSummary{score: score, summary: ToolSummary{
			ID:          entry.descriptor.ID,
			DisplayName: entry.descriptor.DisplayName,
			Description: entry.descriptor.Description,
			Protocols:   append([]Protocol(nil), entry.descriptor.Protocols...),
			Risk:        entry.descriptor.Risk,
			Tags:        append([]string(nil), entry.descriptor.Tags...),
		}})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].score != results[j].score {
			return results[i].score > results[j].score
		}
		return results[i].summary.ID.String() < results[j].summary.ID.String()
	})
	if len(results) > budget.MaxCandidates {
		results = results[:budget.MaxCandidates]
	}
	summaries := make([]ToolSummary, len(results))
	for index := range results {
		summaries[index] = results[index].summary
	}
	return summaries, nil
}

type rankedTool struct {
	entry    catalogEntry
	score    int
	promoted bool
}

func disclosureScore(descriptor ToolDescriptor, query string, promoted, recent, attachmentMatched bool) int {
	score := descriptorMatchScore(descriptor, strings.ToLower(strings.TrimSpace(query)))
	if descriptor.Disclosure == DisclosureAlways {
		score += 100
	}
	if descriptor.Disclosure == DisclosureAttachment && attachmentMatched {
		score += 60
	}
	if promoted {
		score += 1000
	}
	if recent {
		score += 20
	}
	return score
}

func descriptorMatchesAttachment(descriptor ToolDescriptor, attachment *AttachmentSnapshot) bool {
	if len(descriptor.Protocols) == 0 || containsProtocol(descriptor.Protocols, ProtocolAny) {
		return len(descriptor.Capabilities) == 0 || attachmentHasCapabilities(attachment, descriptor.Capabilities)
	}
	if attachment == nil || !containsProtocol(descriptor.Protocols, attachment.Protocol) {
		return false
	}
	return attachmentHasCapabilities(attachment, descriptor.Capabilities)
}

func matchingAttachment(attachments []AttachmentSnapshot, primary *AttachmentSnapshot, descriptor ToolDescriptor) *AttachmentSnapshot {
	if primary != nil && descriptorMatchesAttachment(descriptor, primary) {
		return primary
	}
	for index := range attachments {
		if descriptorMatchesAttachment(descriptor, &attachments[index]) {
			return &attachments[index]
		}
	}
	return nil
}

func findPrimaryAttachment(attachments []AttachmentSnapshot, primaryID string) *AttachmentSnapshot {
	if primaryID != "" {
		for index := range attachments {
			if attachments[index].ID == primaryID {
				return &attachments[index]
			}
		}
	}
	if len(attachments) > 0 {
		return &attachments[0]
	}
	return nil
}

func attachmentHasCapabilities(attachment *AttachmentSnapshot, required []Capability) bool {
	if len(required) == 0 {
		return true
	}
	if attachment == nil {
		return false
	}
	have := make(map[Capability]struct{}, len(attachment.Capabilities))
	for _, capability := range attachment.Capabilities {
		have[capability] = struct{}{}
	}
	for _, capability := range required {
		if _, ok := have[capability]; !ok {
			return false
		}
	}
	return true
}

func containsProtocol(protocols []Protocol, target Protocol) bool {
	for _, protocol := range protocols {
		if protocol == target {
			return true
		}
	}
	return false
}

func normalizeBudget(budget DisclosureBudget) DisclosureBudget {
	defaults := DefaultDisclosureBudget()
	if budget.MaxAlwaysVisible <= 0 {
		budget.MaxAlwaysVisible = defaults.MaxAlwaysVisible
	}
	if budget.MaxCandidates <= 0 {
		budget.MaxCandidates = defaults.MaxCandidates
	}
	if budget.MaxSchemas <= 0 {
		budget.MaxSchemas = defaults.MaxSchemas
	}
	if budget.MaxTokens <= 0 {
		budget.MaxTokens = defaults.MaxTokens
	}
	return budget
}

func toolIDSet(ids []ToolID) map[ToolID]struct{} {
	result := make(map[ToolID]struct{}, len(ids))
	for _, id := range ids {
		result[id] = struct{}{}
	}
	return result
}

func estimateDescriptorTokens(descriptor ToolDescriptor) int {
	bytes := len(descriptor.ID.ModelName()) + len(descriptor.DisplayName) + len(descriptor.Description) +
		len(descriptor.WhenToUse) + len(descriptor.InputSchema)
	for _, tag := range descriptor.Tags {
		bytes += len(tag)
	}
	return max(1, (bytes+3)/4)
}

func newSnapshotID() (string, error) {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return "tls_" + hex.EncodeToString(value[:]), nil
}
