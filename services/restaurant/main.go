package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

type MenuItem struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PriceCents int    `json:"price_cents"`
	Available  bool   `json:"available"`
}

type Restaurant struct {
	ID    string     `json:"id"`
	Name  string     `json:"name"`
	Items []MenuItem `json:"items"`
}

type Server struct {
	restaurants []Restaurant
	byID        map[string]Restaurant
}

func NewServer() *Server {
	restaurants := seedRestaurants()
	byID := make(map[string]Restaurant, len(restaurants))
	for _, restaurant := range restaurants {
		byID[restaurant.ID] = restaurant
	}
	return &Server{restaurants: restaurants, byID: byID}
}

func seedRestaurants() []Restaurant {
	return []Restaurant{
		{
			ID:   "r1",
			Name: "Dhaka Kitchen",
			Items: []MenuItem{
				{ID: "m1", Name: "Chicken Biryani", PriceCents: 35000, Available: true},
				{ID: "m2", Name: "Seekh Kebab", PriceCents: 28000, Available: true},
				{ID: "m3", Name: "Mango Lassi", PriceCents: 12000, Available: true},
			},
		},
		{
			ID:   "r2",
			Name: "Chattogram Cafe",
			Items: []MenuItem{
				{ID: "m4", Name: "Beef Kala Bhuna", PriceCents: 42000, Available: true},
				{ID: "m5", Name: "Paratha", PriceCents: 9000, Available: true},
				{ID: "m6", Name: "Faluda", PriceCents: 15000, Available: true},
			},
		},
		{
			ID:   "r3",
			Name: "Sylhet Grill",
			Items: []MenuItem{
				{ID: "m7", Name: "Grilled Chicken", PriceCents: 39000, Available: true},
				{ID: "m8", Name: "Garlic Naan", PriceCents: 8000, Available: true},
				{ID: "m9", Name: "Lime Soda", PriceCents: 7000, Available: true},
			},
		},
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /restaurants", s.listRestaurants)
	mux.HandleFunc("GET /restaurants/{id}", s.getRestaurant)
	return withRequestLog(mux)
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listRestaurants(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"restaurants": s.restaurants})
}

func (s *Server) getRestaurant(w http.ResponseWriter, r *http.Request) {
	restaurant, ok := s.byID[r.PathValue("id")]
	if !ok {
		writeError(w, http.StatusNotFound, "restaurant_not_found")
		return
	}
	writeJSON(w, http.StatusOK, restaurant)
}

func withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := r.Header.Get("X-Correlation-ID")
		if correlationID == "" {
			correlationID = newID()
		}
		r.Header.Set("X-Correlation-ID", correlationID)
		w.Header().Set("X-Correlation-ID", correlationID)
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("cid=%s method=%s path=%s duration_ms=%d", correlationID, r.Method, r.URL.Path, time.Since(start).Milliseconds())
	})
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
		port = "8081"
	}
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           NewServer().Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("restaurant-service listening on :%s", port)
	log.Fatal(server.ListenAndServe())
}
