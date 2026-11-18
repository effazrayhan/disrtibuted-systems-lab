package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type menuItem struct {
	ID        string `json:"id"`
	Available bool   `json:"available"`
}

type restaurantResponse struct {
	ID    string     `json:"id"`
	Items []menuItem `json:"items"`
}

type orderItem struct {
	MenuItemID string `json:"menu_item_id"`
	Quantity   int    `json:"quantity"`
}

var errRestaurantNotFound = errors.New("restaurant not found")

type createOrderRequest struct {
	RestaurantID string      `json:"restaurant_id"`
	Items        []orderItem `json:"items"`
}

type Order struct {
	OrderID       string      `json:"order_id"`
	CorrelationID string      `json:"correlation_id"`
	Status        string      `json:"status"`
	CreatedAt     time.Time   `json:"created_at"`
	RestaurantID  string      `json:"restaurant_id"`
	Items         []orderItem `json:"items"`
}

type Store struct {
	mu     sync.RWMutex
	orders map[string]Order
}

func NewStore() *Store {
	return &Store{orders: make(map[string]Order)}
}

func (s *Store) Put(order Order) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[order.OrderID] = order
}

func (s *Store) Get(id string) (Order, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	order, ok := s.orders[id]
	return order, ok
}

type RestaurantClient struct {
	baseURL string
	timeout time.Duration
}

func NewRestaurantClient(baseURL string, timeout time.Duration) *RestaurantClient {
	return &RestaurantClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		timeout: timeout,
	}
}

func (c *RestaurantClient) Get(ctx context.Context, correlationID, restaurantID string) (restaurantResponse, error) {
	requestCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	target := c.baseURL + "/restaurants/" + url.PathEscape(restaurantID)
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, target, nil)
	if err != nil {
		return restaurantResponse{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-Correlation-ID", correlationID)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return restaurantResponse{}, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return restaurantResponse{}, errRestaurantNotFound
	}
	if response.StatusCode != http.StatusOK {
		return restaurantResponse{}, fmt.Errorf("restaurant service returned status %d", response.StatusCode)
	}

	var restaurant restaurantResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&restaurant); err != nil {
		return restaurantResponse{}, err
	}
	if restaurant.ID == "" {
		return restaurantResponse{}, errors.New("restaurant response missing id")
	}
	return restaurant, nil
}

type Server struct {
	client *RestaurantClient
	store  *Store
}

func NewServer(client *RestaurantClient) *Server {
	return &Server{client: client, store: NewStore()}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("POST /orders", s.createOrder)
	mux.HandleFunc("GET /orders/{id}", s.getOrder)
	return withRequestLog(mux)
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) createOrder(w http.ResponseWriter, r *http.Request) {
	correlationID := correlationID(r)

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var request createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "malformed_json")
		return
	}
	if request.RestaurantID == "" || len(request.Items) == 0 {
		writeError(w, http.StatusBadRequest, "missing_fields")
		return
	}

	seen := make(map[string]bool, len(request.Items))
	for _, item := range request.Items {
		if item.MenuItemID == "" || item.Quantity <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_item")
			return
		}
		if seen[item.MenuItemID] {
			writeError(w, http.StatusBadRequest, "duplicate_item")
			return
		}
		seen[item.MenuItemID] = true
	}

	restaurant, err := s.client.Get(r.Context(), correlationID, request.RestaurantID)
	if err != nil {
		if errors.Is(err, errRestaurantNotFound) {
			writeError(w, http.StatusNotFound, "restaurant_not_found")
			return
		}
		log.Printf("cid=%s event=dependency_error error=%v", correlationID, err)
		writeError(w, http.StatusServiceUnavailable, "dependency_unavailable")
		return
	}

	available := make(map[string]bool, len(restaurant.Items))
	for _, item := range restaurant.Items {
		if item.Available {
			available[item.ID] = true
		}
	}
	for _, item := range request.Items {
		if !available[item.MenuItemID] {
			writeError(w, http.StatusNotFound, "menu_item_not_found")
			return
		}
	}

	order := Order{
		OrderID:       newID(),
		CorrelationID: correlationID,
		Status:        "created",
		CreatedAt:     time.Now().UTC(),
		RestaurantID:  request.RestaurantID,
		Items:         request.Items,
	}
	s.store.Put(order)
	log.Printf("cid=%s order_id=%s restaurant_id=%s status=created", correlationID, order.OrderID, order.RestaurantID)
	writeJSON(w, http.StatusCreated, order)
}

func (s *Server) getOrder(w http.ResponseWriter, r *http.Request) {
	order, ok := s.store.Get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "order_not_found")
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := correlationID(r)
		r.Header.Set("X-Correlation-ID", correlationID)
		w.Header().Set("X-Correlation-ID", correlationID)
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("cid=%s method=%s path=%s duration_ms=%d", correlationID, r.Method, r.URL.Path, time.Since(start).Milliseconds())
	})
}

func correlationID(r *http.Request) string {
	if value := r.Header.Get("X-Correlation-ID"); value != "" {
		return value
	}
	return newID()
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write_json_error=%v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

func newID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buffer)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}
	restaurantURL := os.Getenv("RESTAURANT_URL")
	if restaurantURL == "" {
		log.Fatal("RESTAURANT_URL must be set")
	}
	timeout := 2 * time.Second
	if value := os.Getenv("RESTAURANT_TIMEOUT_MS"); value != "" {
		milliseconds, err := strconv.Atoi(value)
		if err != nil || milliseconds <= 0 {
			log.Fatal("RESTAURANT_TIMEOUT_MS must be a positive integer")
		}
		timeout = time.Duration(milliseconds) * time.Millisecond
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           NewServer(NewRestaurantClient(restaurantURL, timeout)).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("order-service listening on :%s restaurant_url=%s timeout=%s", port, restaurantURL, timeout)
	log.Fatal(server.ListenAndServe())
}
