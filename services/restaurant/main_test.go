package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	NewServer().Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("X-Correlation-ID"); got == "" {
		t.Fatal("missing correlation id header")
	}
}

func TestListRestaurants(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/restaurants", nil)
	NewServer().Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var body struct {
		Restaurants []Restaurant `json:"restaurants"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Restaurants) < 3 {
		t.Fatalf("restaurants = %d, want >= 3", len(body.Restaurants))
	}
	items := 0
	for _, restaurant := range body.Restaurants {
		items += len(restaurant.Items)
	}
	if items < 6 {
		t.Fatalf("items = %d, want >= 6", items)
	}
}

func TestGetRestaurant(t *testing.T) {
	router := NewServer().Routes()

	t.Run("found", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/restaurants/r1", nil)
		router.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		var restaurant Restaurant
		if err := json.Unmarshal(recorder.Body.Bytes(), &restaurant); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if restaurant.ID != "r1" {
			t.Fatalf("id = %q, want r1", restaurant.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/restaurants/does-not-exist", nil)
		router.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
		}
	})
}
