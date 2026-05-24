package jira

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_TestConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/myself" {
			t.Errorf("expected path /rest/api/3/myself, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accountId":"123"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@example.com", "token")
	err := client.TestConnection()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_DetectStoryPointsField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{"id":"customfield_101", "name":"Sprint"},
			{"id":"customfield_storypoints", "name":"Story Points"}
		]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	field, err := client.DetectStoryPointsField()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if field != "customfield_storypoints" {
		t.Errorf("expected customfield_storypoints, got %s", field)
	}
}

func TestClient_FetchRetroData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/search/jql" {
			// Mock search response
			resp := struct {
				Issues []map[string]interface{} `json:"issues"`
				Total  int                      `json:"total"`
			}{
				Total: 1,
				Issues: []map[string]interface{}{
					{
						"key": "PROJ-101",
						"fields": map[string]interface{}{
							"summary": "Sample Task",
							"status": map[string]interface{}{
								"name": "Done",
								"statusCategory": map[string]interface{}{
									"name": "Done",
									"key":  "done",
								},
							},
							"timespent":                    36000,
							"statuscategorychangeddate":    "2026-05-10T10:00:00.000Z",
							"customfield_storypoints":      3.0,
							"worklog": map[string]interface{}{
								"total":      2,
								"maxResults": 20,
								"worklogs": []map[string]interface{}{
									{
										"author": map[string]interface{}{
											"displayName": "Alice",
										},
										"timeSpentSeconds": 18000, // 5 hours
										"started":          "2026-05-05T09:00:00.000Z", // inside range
									},
									{
										"author": map[string]interface{}{
											"displayName": "Bob",
										},
										"timeSpentSeconds": 7200, // 2 hours
										"started":          "2026-04-28T09:00:00.000Z", // OUTSIDE range (starts before May 1)
									},
								},
							},
						},
					},
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	data, err := client.FetchRetroData("project = PROJ", "2026-05-01", "2026-05-15", "customfield_storypoints")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if data.Totals.TotalStoryPoints != 3.0 {
		t.Errorf("expected 3.0 story points, got %f", data.Totals.TotalStoryPoints)
	}

	// Bob should be filtered out because his worklog was dated April 28, which is outside the May 1–15 range.
	// Total hours logged should be 5.0 (only Alice).
	if data.Totals.TotalHoursLogged != 5.0 {
		t.Errorf("expected 5.0 hours logged, got %f", data.Totals.TotalHoursLogged)
	}

	if len(data.Leaderboard) != 1 {
		t.Fatalf("expected leaderboard to have 1 entry (Alice), got %d", len(data.Leaderboard))
	}

	if data.Leaderboard[0].AuthorName != "Alice" {
		t.Errorf("expected Alice in leaderboard, got %s", data.Leaderboard[0].AuthorName)
	}

	if data.Leaderboard[0].HoursLogged != 5.0 {
		t.Errorf("expected Alice to have 5.0 hours logged, got %f", data.Leaderboard[0].HoursLogged)
	}
}
