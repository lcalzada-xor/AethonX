package common

import (
	"encoding/json"
	"sync"
	"testing"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
)

// Test response struct
type testResponse struct {
	Host   string `json:"host"`
	Status int    `json:"status"`
}

func TestJSONLineHandler_ProcessLine(t *testing.T) {
	logger := logx.New()
	target := domain.Target{Root: "example.com"}
	handler := NewJSONLineHandler[testResponse](logger, target)

	// Valid JSON
	line := []byte(`{"host":"sub.example.com","status":200}`)
	if err := handler.ProcessLine(line); err != nil {
		t.Errorf("ProcessLine failed: %v", err)
	}

	responses := handler.GetResponses()
	if len(responses) != 1 {
		t.Errorf("Expected 1 response, got %d", len(responses))
	}

	if responses[0].Host != "sub.example.com" {
		t.Errorf("Expected host 'sub.example.com', got %s", responses[0].Host)
	}

	if responses[0].Status != 200 {
		t.Errorf("Expected status 200, got %d", responses[0].Status)
	}
}

func TestJSONLineHandler_ProcessLine_InvalidJSON(t *testing.T) {
	logger := logx.New()
	target := domain.Target{Root: "example.com"}
	handler := NewJSONLineHandler[testResponse](logger, target)

	// Invalid JSON should not cause error
	line := []byte(`{invalid json}`)
	if err := handler.ProcessLine(line); err != nil {
		t.Errorf("ProcessLine should not return error for invalid JSON, got: %v", err)
	}

	responses := handler.GetResponses()
	if len(responses) != 0 {
		t.Errorf("Expected 0 responses after invalid JSON, got %d", len(responses))
	}
}

func TestJSONLineHandler_ProcessLine_Multiple(t *testing.T) {
	logger := logx.New()
	target := domain.Target{Root: "example.com"}
	handler := NewJSONLineHandler[testResponse](logger, target)

	// Process multiple lines
	lines := []string{
		`{"host":"sub1.example.com","status":200}`,
		`{"host":"sub2.example.com","status":301}`,
		`{"host":"sub3.example.com","status":404}`,
	}

	for _, line := range lines {
		if err := handler.ProcessLine([]byte(line)); err != nil {
			t.Errorf("ProcessLine failed: %v", err)
		}
	}

	responses := handler.GetResponses()
	if len(responses) != 3 {
		t.Errorf("Expected 3 responses, got %d", len(responses))
	}

	// Verify all responses
	expectedHosts := []string{"sub1.example.com", "sub2.example.com", "sub3.example.com"}
	expectedStatuses := []int{200, 301, 404}

	for i, resp := range responses {
		if resp.Host != expectedHosts[i] {
			t.Errorf("Response %d: expected host %s, got %s", i, expectedHosts[i], resp.Host)
		}
		if resp.Status != expectedStatuses[i] {
			t.Errorf("Response %d: expected status %d, got %d", i, expectedStatuses[i], resp.Status)
		}
	}
}

func TestJSONLineHandler_ProcessLine_Concurrent(t *testing.T) {
	logger := logx.New()
	target := domain.Target{Root: "example.com"}
	handler := NewJSONLineHandler[testResponse](logger, target)

	// Process lines concurrently
	var wg sync.WaitGroup
	numGoroutines := 10
	linesPerGoroutine := 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < linesPerGoroutine; j++ {
				line, _ := json.Marshal(testResponse{
					Host:   "sub.example.com",
					Status: id*100 + j,
				})
				handler.ProcessLine(line)
			}
		}(i)
	}

	wg.Wait()

	responses := handler.GetResponses()
	expected := numGoroutines * linesPerGoroutine
	if len(responses) != expected {
		t.Errorf("Expected %d responses, got %d", expected, len(responses))
	}
}

func TestJSONLineHandler_GetResponseCount(t *testing.T) {
	logger := logx.New()
	target := domain.Target{Root: "example.com"}
	handler := NewJSONLineHandler[testResponse](logger, target)

	if count := handler.GetResponseCount(); count != 0 {
		t.Errorf("Expected initial count 0, got %d", count)
	}

	handler.ProcessLine([]byte(`{"host":"sub.example.com","status":200}`))

	if count := handler.GetResponseCount(); count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}

	handler.ProcessLine([]byte(`{"host":"sub2.example.com","status":301}`))

	if count := handler.GetResponseCount(); count != 2 {
		t.Errorf("Expected count 2, got %d", count)
	}
}

func TestJSONLineHandler_Finalize(t *testing.T) {
	logger := logx.New()
	target := domain.Target{Root: "example.com"}
	handler := NewJSONLineHandler[testResponse](logger, target)

	// Add some responses
	handler.ProcessLine([]byte(`{"host":"sub.example.com","status":200}`))
	handler.ProcessLine([]byte(`{"host":"sub2.example.com","status":301}`))

	// Finalize should not error
	if err := handler.Finalize(); err != nil {
		t.Errorf("Finalize failed: %v", err)
	}

	// Responses should still be accessible after finalize
	responses := handler.GetResponses()
	if len(responses) != 2 {
		t.Errorf("Expected 2 responses after finalize, got %d", len(responses))
	}
}

func TestJSONLineHandler_GetResponses_Copy(t *testing.T) {
	logger := logx.New()
	target := domain.Target{Root: "example.com"}
	handler := NewJSONLineHandler[testResponse](logger, target)

	handler.ProcessLine([]byte(`{"host":"sub.example.com","status":200}`))

	// Get responses
	responses1 := handler.GetResponses()
	responses2 := handler.GetResponses()

	// Verify they are separate copies
	if &responses1[0] == &responses2[0] {
		t.Error("GetResponses should return a copy, not the same slice")
	}

	// Modify first copy
	responses1[0].Host = "modified.example.com"

	// Second copy should not be affected
	if responses2[0].Host == "modified.example.com" {
		t.Error("Modifying returned slice affected internal state")
	}

	// Original responses should not be affected
	responses3 := handler.GetResponses()
	if responses3[0].Host != "sub.example.com" {
		t.Error("Internal state was modified when it shouldn't be")
	}
}

// Test with complex nested struct
type complexResponse struct {
	Host     string            `json:"host"`
	Status   int               `json:"status"`
	Headers  map[string]string `json:"headers"`
	Tags     []string          `json:"tags"`
	Metadata struct {
		SSL       bool   `json:"ssl"`
		Redirects int    `json:"redirects"`
		Title     string `json:"title"`
	} `json:"metadata"`
}

func TestJSONLineHandler_ComplexType(t *testing.T) {
	logger := logx.New()
	target := domain.Target{Root: "example.com"}
	handler := NewJSONLineHandler[complexResponse](logger, target)

	jsonLine := `{
		"host":"sub.example.com",
		"status":200,
		"headers":{"content-type":"text/html","server":"nginx"},
		"tags":["wordpress","ssl"],
		"metadata":{"ssl":true,"redirects":2,"title":"Example Site"}
	}`

	if err := handler.ProcessLine([]byte(jsonLine)); err != nil {
		t.Errorf("ProcessLine failed: %v", err)
	}

	responses := handler.GetResponses()
	if len(responses) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(responses))
	}

	resp := responses[0]

	if resp.Host != "sub.example.com" {
		t.Errorf("Expected host 'sub.example.com', got %s", resp.Host)
	}

	if len(resp.Headers) != 2 {
		t.Errorf("Expected 2 headers, got %d", len(resp.Headers))
	}

	if len(resp.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(resp.Tags))
	}

	if !resp.Metadata.SSL {
		t.Error("Expected SSL to be true")
	}

	if resp.Metadata.Redirects != 2 {
		t.Errorf("Expected 2 redirects, got %d", resp.Metadata.Redirects)
	}
}
