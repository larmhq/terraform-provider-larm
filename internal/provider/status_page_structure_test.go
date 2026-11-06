package provider

import (
	"testing"
)

func TestComponentsListFromAPI(t *testing.T) {
	description := "Carries app traffic."
	entries := []treeEntry{
		{
			Type:        "component",
			ID:          "component-id",
			Name:        "Web",
			Description: &description,
			Monitors:    []monitorEntry{{MonitorID: "monitor-id", DownStatus: "major_outage"}},
		},
		{
			Type: "group",
			ID:   "group-id",
			Name: "Core",
			Components: []treeEntry{
				{ID: "child-id", Name: "API"},
			},
		},
	}

	list, diags := componentsListFromAPI(entries)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got := len(list.Elements()); got != 2 {
		t.Fatalf("got %d entries, want 2", got)
	}
}

func TestComponentsListFromAPI_emptyListsAreNotNull(t *testing.T) {
	list, diags := componentsListFromAPI(nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if list.IsNull() || len(list.Elements()) != 0 {
		t.Fatalf("got %s, want []", list)
	}

	if monitors := monitorListValue(nil); monitors.IsNull() || len(monitors.Elements()) != 0 {
		t.Fatalf("got %s, want []", monitors)
	}
}
