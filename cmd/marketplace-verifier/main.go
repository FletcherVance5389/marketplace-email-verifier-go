package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/example/marketplace-email-verifier/internal/marketplace"
)

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	publicURL := envOr("PUBLIC_URL", "http://localhost:8080")
	address := envOr("LISTEN_ADDR", ":8080")
	flow := marketplace.NewSignupFlow(marketplace.InfraiEmailSender{
		APIKey:     apiKey,
		MaxRetries: 3,
	}, publicURL)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /signup", func(w http.ResponseWriter, r *http.Request) {
		var signup marketplace.Signup
		if err := json.NewDecoder(r.Body).Decode(&signup); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		if signup.SellerEmail == "" || signup.SellerName == "" || signup.BuyerEmail == "" || signup.OrderID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "seller_email, seller_name, buyer_email, and order_id are required"})
			return
		}
		_, messageID, err := flow.Start(r.Context(), signup)
		if err != nil {
			var apiErr *marketplace.InfraiError
			if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
				writeJSON(w, apiErr.StatusCode, map[string]string{"error": apiErr.Message})
				return
			}
			log.Printf("send verification: %v", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "verification email could not be sent"})
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "verification_sent", "message_id": messageID})
	})
	mux.HandleFunc("GET /verify", func(w http.ResponseWriter, r *http.Request) {
		state, err := flow.Verify(r.URL.Query().Get("token"))
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "verification link is invalid"})
			return
		}
		writeJSON(w, http.StatusOK, state)
	})

	log.Printf("marketplace verifier listening on %s", address)
	log.Fatal(http.ListenAndServe(address, mux))
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}
