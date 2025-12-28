package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestWikiStatus(t *testing.T) {
	tests := []struct {
		status   WikiStatus
		expected string
	}{
		{WikiStatusPending, "pending"},
		{WikiStatusOK, "ok"},
		{WikiStatusError, "error"},
		{WikiStatusOffline, "offline"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.status)
			}
		})
	}
}

func TestWikiJSONSerialization(t *testing.T) {
	now := time.Now()
	wikiID := uuid.New()
	wikiName := "Test Wiki"
	sitename := "Test Site"
	url := "https://example.com"

	wiki := Wiki{
		ID:              wikiID,
		URL:             url,
		WikiName:        &wikiName,
		Sitename:        &sitename,
		Status:          WikiStatusOK,
		HasArchive:      true,
		APIAvailable:    true,
		CreatedAt:       now,
		UpdatedAt:       now,
		IsActive:        true,
	}

	// Test JSON marshaling
	data, err := json.Marshal(wiki)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Check that JSON contains expected fields (snake_case)
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result["id"] != wikiID.String() {
		t.Errorf("Expected id %s, got %v", wikiID.String(), result["id"])
	}

	if result["url"] != url {
		t.Errorf("Expected url %s, got %v", url, result["url"])
	}

	if result["wiki_name"] != wikiName {
		t.Errorf("Expected wiki_name %s, got %v", wikiName, result["wiki_name"])
	}

	if result["status"] != "ok" {
		t.Errorf("Expected status ok, got %v", result["status"])
	}

	if result["has_archive"] != true {
		t.Errorf("Expected has_archive true, got %v", result["has_archive"])
	}

	if result["api_available"] != true {
		t.Errorf("Expected api_available true, got %v", result["api_available"])
	}
}

func TestWikiStatsJSONSerialization(t *testing.T) {
	now := time.Now()
	wikiID := uuid.New()
	responseTime := 150
	httpStatus := 200

	stats := WikiStats{
		WikiID:        wikiID,
		Time:          now,
		Pages:         1000,
		Articles:      500,
		Edits:         5000,
		Images:        200,
		Users:         100,
		ActiveUsers:   10,
		Admins:        5,
		Jobs:          3,
		ResponseTimeMs: &responseTime,
		HTTPStatus:    &httpStatus,
	}

	// Test JSON marshaling
	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Check snake_case field names
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result["wiki_id"] != wikiID.String() {
		t.Errorf("Expected wiki_id %s, got %v", wikiID.String(), result["wiki_id"])
	}

	if result["pages"] != float64(1000) {
		t.Errorf("Expected pages 1000, got %v", result["pages"])
	}

	if result["response_time_ms"] != float64(150) {
		t.Errorf("Expected response_time_ms 150, got %v", result["response_time_ms"])
	}
}

func TestWikiArchiveJSONSerialization(t *testing.T) {
	now := time.Now()
	wikiID := uuid.New()
	itemSize := int64(1234567890)

	archive := WikiArchive{
		WikiID:           wikiID,
		IAIdentifier:     "wiki-example-20240101",
		AddedDate:        &now,
		DumpDate:         &now,
		ItemSize:         &itemSize,
		HasXMLCurrent:    true,
		HasXMLHistory:    true,
		HasImagesDump:    false,
		HasTitlesList:    true,
		HasImagesList:    false,
		HasLegacyWikidump: false,
	}

	// Test JSON marshaling
	data, err := json.Marshal(archive)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Check snake_case field names
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result["wiki_id"] != wikiID.String() {
		t.Errorf("Expected wiki_id %s, got %v", wikiID.String(), result["wiki_id"])
	}

	if result["ia_identifier"] != "wiki-example-20240101" {
		t.Errorf("Expected ia_identifier wiki-example-20240101, got %v", result["ia_identifier"])
	}

	if result["has_xml_current"] != true {
		t.Errorf("Expected has_xml_current true, got %v", result["has_xml_current"])
	}

	if result["item_size"] != float64(itemSize) {
		t.Errorf("Expected item_size %d, got %v", itemSize, result["item_size"])
	}
}

func TestWikiPointerFields(t *testing.T) {
	// Test that pointer fields work correctly
	wiki1 := "My Wiki"
	sitename := "My Site"

	wiki := Wiki{
		URL:      "https://example.com",
		WikiName: &wiki1,
		Sitename: &sitename,
		Status:   WikiStatusOK,
	}

	if wiki.WikiName == nil {
		t.Error("Expected WikiName to be set")
	}

	if *wiki.WikiName != "My Wiki" {
		t.Errorf("Expected WikiName 'My Wiki', got %s", *wiki.WikiName)
	}

	// Test nil pointers
	wiki2 := Wiki{
		URL:    "https://example2.com",
		Status: WikiStatusPending,
	}

	if wiki2.WikiName != nil {
		t.Error("Expected WikiName to be nil")
	}

	if wiki2.Sitename != nil {
		t.Error("Expected Sitename to be nil")
	}
}
