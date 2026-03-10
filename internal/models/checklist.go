package models

import "time"

// Checklist status constants.
const (
	ChecklistStatusPending = "pending"
	ChecklistStatusDone    = "done"
)

// Checklist group constants.
const (
	ChecklistGroupLogistics         = "logistics"
	ChecklistGroupMarketingApproval = "marketing_approval"
	ChecklistGroupMarketingInternal = "marketing_internal"
	ChecklistGroupOptionalRequests  = "optional_requests"
)

// ChecklistItemDef defines a known checklist item in the catalog.
type ChecklistItemDef struct {
	Key           string // DB key, e.g. "logistics_space_reservation"
	Label         string // Display label
	Group         string // One of the ChecklistGroup* constants
	NeedsApproval bool   // true = "Pending/Approved", false = "Not Done/Done"
	Condition     string // "venue_jamatkhana" or "" (always shown)
}

// ChecklistItem is a DB row from event_checklist_items.
type ChecklistItem struct {
	ID        string
	EventID   string
	ItemKey   string
	Status    string // "pending" or "done"
	UpdatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// IsDone returns true when the item has been completed/approved.
func (c *ChecklistItem) IsDone() bool {
	return c.Status == ChecklistStatusDone
}

// ChecklistGroupData holds one group's items for template rendering.
type ChecklistGroupData struct {
	Label      string              // "Logistics", "Marketing (Approval Required)", etc.
	GroupKey   string              // group constant
	Items      []ChecklistItemView // items in this group
	Selectable bool                // true = admin chooses which items to add
}

// ChecklistItemView combines the definition with the current DB state.
type ChecklistItemView struct {
	Def    ChecklistItemDef
	Active bool   // true = row exists in DB for this event
	Status string // "pending" or "done" (from DB; "pending" if not active)
	ID     string // DB row ID (empty if not active)
}

// IsDone returns whether the view item is done.
func (v ChecklistItemView) IsDone() bool {
	return v.Status == ChecklistStatusDone
}

// StatusLabel returns a user-facing label.
func (v ChecklistItemView) StatusLabel() string {
	if !v.Active {
		return ""
	}
	if v.Def.NeedsApproval {
		if v.Status == ChecklistStatusDone {
			return "Approved"
		}
		return "Pending"
	}
	if v.Status == ChecklistStatusDone {
		return "Done"
	}
	return "Not Done"
}

// StatusBadgeClass returns Tailwind classes for the status badge.
func (v ChecklistItemView) StatusBadgeClass() string {
	if !v.Active {
		return ""
	}
	if v.Status == ChecklistStatusDone {
		return "bg-green-100 text-green-800"
	}
	return "bg-yellow-100 text-yellow-800"
}

// ChecklistCatalog is the ordered list of all known checklist items.
var ChecklistCatalog = []ChecklistItemDef{
	// ── Logistics (auto-populated on approval) ──
	{Key: "logistics_space_reservation", Label: "Space Reservation", Group: ChecklistGroupLogistics, NeedsApproval: true, Condition: "venue_jamatkhana"},
	{Key: "logistics_flyer_approval", Label: "Flyer Approval", Group: ChecklistGroupLogistics, NeedsApproval: true},
	{Key: "logistics_pdt_food_request", Label: "PDT Food Request", Group: ChecklistGroupLogistics, NeedsApproval: true},
	{Key: "logistics_council_calendar", Label: "Council Calendar Approval", Group: ChecklistGroupLogistics, NeedsApproval: true},

	// ── Marketing — Approval Required (optional, admin selects) ──
	{Key: "marketing_approval_ismaili_insight", Label: "Ismaili Insight", Group: ChecklistGroupMarketingApproval, NeedsApproval: true},
	{Key: "marketing_approval_post_event_social", Label: "Post-Event Social Media", Group: ChecklistGroupMarketingApproval, NeedsApproval: true},
	{Key: "marketing_approval_the_ismaili", Label: "the.ismaili Social Media", Group: ChecklistGroupMarketingApproval, NeedsApproval: true},
	{Key: "marketing_approval_se_council_whatsapp", Label: "Southeast Council WhatsApp", Group: ChecklistGroupMarketingApproval, NeedsApproval: true},

	// ── Marketing — Internal (auto-populated, no approval needed) ──
	{Key: "marketing_internal_ipn_whatsapp", Label: "IPN Southeast WhatsApp", Group: ChecklistGroupMarketingInternal, NeedsApproval: false},
	{Key: "marketing_internal_ipn_instagram", Label: "IPN Southeast Instagram", Group: ChecklistGroupMarketingInternal, NeedsApproval: false},
	{Key: "marketing_internal_ipn_facebook", Label: "IPN Southeast Facebook", Group: ChecklistGroupMarketingInternal, NeedsApproval: false},
	{Key: "marketing_internal_ipn_linkedin", Label: "IPN Southeast LinkedIn", Group: ChecklistGroupMarketingInternal, NeedsApproval: false},

	// ── Optional Requests (admin selects, need approval) ──
	{Key: "optional_photo_video", Label: "Photo/Video", Group: ChecklistGroupOptionalRequests, NeedsApproval: true},
	{Key: "optional_av", Label: "AV", Group: ChecklistGroupOptionalRequests, NeedsApproval: true},
	{Key: "optional_live_social", Label: "Live Social Media", Group: ChecklistGroupOptionalRequests, NeedsApproval: true},
}

// ChecklistGroups returns the group metadata in display order.
func ChecklistGroups() []ChecklistGroupData {
	return []ChecklistGroupData{
		{Label: "Logistics", GroupKey: ChecklistGroupLogistics, Selectable: false},
		{Label: "Marketing (Approval Required)", GroupKey: ChecklistGroupMarketingApproval, Selectable: true},
		{Label: "Marketing (Internal)", GroupKey: ChecklistGroupMarketingInternal, Selectable: false},
		{Label: "Optional Requests", GroupKey: ChecklistGroupOptionalRequests, Selectable: true},
	}
}

// CatalogByGroup returns catalog items filtered to a given group.
func CatalogByGroup(group string) []ChecklistItemDef {
	var result []ChecklistItemDef
	for _, def := range ChecklistCatalog {
		if def.Group == group {
			result = append(result, def)
		}
	}
	return result
}
