package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const validRestaurantBody = `{"id":"r1","name":"Dhaka Kitchen","items":[{"id":"m1","name":"Biryani","available":true},{"id":"m2","name":"Kebab","available":true}]}`

func newFakeRestaurant(t *testing.T, status int, body string, calls *atomic.Int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls != nil {
			calls.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
}

func postOrder(body, correlationID string, router http.Handler) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(body))
	if correlationID != "" {
		request.Header.Set("X-Correlation-ID", correlationID)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestHealthz(t *testing.T) {
	router := NewServer(NewRestaurantClient("http://127.0.0.1:1", time.Second)).Routes()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestCreateOrderSuccess(t *testing.T) {
	fake := newFakeRestaurant(t, http.StatusOK, validRestaurantBody, nil)
	defer fake.Close()
	router := NewServer(NewRestaurantClient(fake.URL, time.Second)).Routes()

	recorder := postOrder(`{"restaurant_id":"r1","items":[{"menu_item_id":"m1","quantity":2}]}`, "cid-1", router)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}

	var order Order
	if err := json.Unmarshal(recorder.Body.Bytes(), &order); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if order.OrderID == "" || order.CorrelationID != "cid-1" || order.Status != "created" || order.CreatedAt.IsZero() {
		t.Fatalf("unexpected order: %+v", order)
	}

	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/orders/"+order.OrderID, nil)
	router.ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", getRecorder.Code, http.StatusOK)
	}
}

func TestCreateOrderMalformedJSON(t *testing.T) {
	var calls atomic.Int64
	fake := newFakeRestaurant(t, http.StatusOK, validRestaurantBody, &calls)
	defer fake.Close()
	router := NewServer(NewRestaurantClient(fake.URL, time.Second)).Routes()

	recorder := postOrder(`{"restaurant_id":`, "", router)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if calls.Load() != 0 {
		t.Fatalf("upstream calls = %d, want 0", calls.Load())
	}
}

func TestCreateOrderBadQuantity(t *testing.T) {
	var calls atomic.Int64
	fake := newFakeRestaurant(t, http.StatusOK, validRestaurantBody, &calls)
	defer fake.Close()
	router := NewServer(NewRestaurantClient(fake.URL, time.Second)).Routes()

	recorder := postOrder(`{"restaurant_id":"r1","items":[{"menu_item_id":"m1","quantity":0}]}`, "", router)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if calls.Load() != 0 {
		t.Fatalf("upstream calls = %d, want 0", calls.Load())
	}
}

func TestCreateOrderRestaurantNotFound(t *testing.T) {
	fake := newFakeRestaurant(t, http.StatusNotFound, `{"error":"restaurant_not_found"}`, nil)
	defer fake.Close()
	router := NewServer(NewRestaurantClient(fake.URL, time.Second)).Routes()

	recorder := postOrder(`{"restaurant_id":"missing","items":[{"menu_item_id":"m1","quantity":1}]}`, "", router)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestCreateOrderItemNotFound(t *testing.T) {
	fake := newFakeRestaurant(t, http.StatusOK, validRestaurantBody, nil)
	defer fake.Close()
	router := NewServer(NewRestaurantClient(fake.URL, time.Second)).Routes()

	recorder := postOrder(`{"restaurant_id":"r1","items":[{"menu_item_id":"missing","quantity":1}]}`, "", router)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestCreateOrderDependencyUnavailable(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer slow.Close()

	router := NewServer(NewRestaurantClient(slow.URL, 100*time.Millisecond)).Routes()
	start := time.Now()
	recorder := postOrder(`{"restaurant_id":"r1","items":[{"menu_item_id":"m1","quantity":1}]}`, "", router)
	elapsed := time.Since(start)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d body=%s", recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
	}
	if elapsed > 2*time.Second {
		t.Fatalf("elapsed = %s, want < 2s", elapsed)
	}
}

func TestGetOrderNotFound(t *testing.T) {
	router := NewServer(NewRestaurantClient("http://127.0.0.1:1", time.Second)).Routes()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/orders/missing", nil))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestConcurrentOrdersAreUnique(t *testing.T) {
	fake := newFakeRestaurant(t, http.StatusOK, validRestaurantBody, nil)
	defer fake.Close()
	router := NewServer(NewRestaurantClient(fake.URL, time.Second)).Routes()

	const total = 20
	var waitGroup sync.WaitGroup
	ids := make(chan string, total)
	errors := make(chan string, total)

	for i := 0; i < total; i++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			recorder := postOrder(`{"restaurant_id":"r1","items":[{"menu_item_id":"m1","quantity":1}]}`, "", router)
			if recorder.Code != http.StatusCreated {
				errors <- "unexpected status: " + recorder.Body.String()
				return
			}
			var order Order
			if err := json.Unmarshal(recorder.Body.Bytes(), &order); err != nil {
				errors <- err.Error()
				return
			}
			ids <- order.OrderID
		}()
	}
	waitGroup.Wait()
	close(ids)
	close(errors)

	for message := range errors {
		t.Error(message)
	}
	seen := make(map[string]bool, total)
	for id := range ids {
		if seen[id] {
			t.Errorf("duplicate order id %q", id)
		}
		seen[id] = true
	}
	if len(seen) != total {
		t.Fatalf("unique ids = %d, want %d", len(seen), total)
	}
}
