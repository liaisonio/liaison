package tool

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValidateDescriptor_WhenInvalid_ReturnsContextualError(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ToolDescriptor)
	}{
		{"namespace", func(descriptor *ToolDescriptor) { descriptor.ID.Namespace = "SSH Tools" }},
		{"name", func(descriptor *ToolDescriptor) { descriptor.ID.Name = "Execute-Command" }},
		{"version", func(descriptor *ToolDescriptor) { descriptor.ID.Version = "latest" }},
		{"description", func(descriptor *ToolDescriptor) { descriptor.Description = "" }},
		{"schema", func(descriptor *ToolDescriptor) { descriptor.InputSchema = json.RawMessage(`{`) }},
		{"source", func(descriptor *ToolDescriptor) { descriptor.Source.ID = "" }},
		{"timeout", func(descriptor *ToolDescriptor) { descriptor.DefaultTimeout = -time.Second }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			descriptor := testDescriptor("ssh", "execute", "1.0.0", DisclosureAttachment)
			testCase.mutate(&descriptor)
			err := ValidateDescriptor(descriptor)
			assert.ErrorIs(t, err, ErrInvalidDescriptor)
		})
	}
}

func TestToolSetSnapshot_Find_DoesNotExposeUnknownTool(t *testing.T) {
	descriptor := testDescriptor("ssh", "execute", "1.0.0", DisclosureAttachment)
	snapshot := ToolSetSnapshot{Tools: []ExposedTool{{Descriptor: descriptor}}}

	_, found := snapshot.Find(descriptor.ID)
	assert.True(t, found)
	_, found = snapshot.Find(ToolID{Namespace: "mysql", Name: "query", Version: "1.0.0"})
	assert.False(t, found)
	assert.True(t, errors.Is(ErrToolNotExposed, ErrToolNotExposed))
}

func TestParseToolID_RoundTripsCanonicalID(t *testing.T) {
	want := ToolID{Namespace: "mysql", Name: "query", Version: "1.2.0-rc.1"}
	got, err := ParseToolID(want.String())
	assert.NoError(t, err)
	assert.Equal(t, want, got)

	_, err = ParseToolID("mysql.query")
	assert.ErrorIs(t, err, ErrInvalidDescriptor)
}
