package marketplace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sync"
)

var ErrUnknownVerification = errors.New("verification link is invalid")

type Signup struct {
	SellerEmail string   `json:"seller_email"`
	SellerName  string   `json:"seller_name"`
	AssetIDs    []string `json:"asset_ids"`
	BuyerEmail  string   `json:"buyer_email"`
	OrderID     string   `json:"order_id"`
}

type MarketplaceState struct {
	SellerEmail         string   `json:"seller_email"`
	AssetIDs            []string `json:"asset_ids"`
	BuyerEmail          string   `json:"buyer_email"`
	OrderID             string   `json:"order_id"`
	EmailVerified       bool     `json:"email_verified"`
	BuyerUpdatesEnabled bool     `json:"buyer_updates_enabled"`
	OrderHandoffReady   bool     `json:"order_handoff_ready"`
	MessageID           string   `json:"message_id"`
}

type SignupFlow struct {
	email     EmailSender
	publicURL string
	mu        sync.Mutex
	pending   map[string]MarketplaceState
}

func NewSignupFlow(email EmailSender, publicURL string) *SignupFlow {
	return &SignupFlow{email: email, publicURL: publicURL, pending: make(map[string]MarketplaceState)}
}

func (f *SignupFlow) Start(ctx context.Context, signup Signup) (string, string, error) {
	token, err := newToken()
	if err != nil {
		return "", "", fmt.Errorf("create verification token: %w", err)
	}
	verificationURL := f.publicURL + "/verify?token=" + url.QueryEscape(token)
	messageID, err := f.email.SendVerification(ctx, VerificationEmail{
		To:              signup.SellerEmail,
		SellerName:      signup.SellerName,
		VerificationURL: verificationURL,
		IdempotencyKey:  "marketplace-signup-" + signup.OrderID,
	})
	if err != nil {
		return "", "", err
	}

	f.mu.Lock()
	f.pending[token] = MarketplaceState{
		SellerEmail: signup.SellerEmail,
		AssetIDs:    append([]string(nil), signup.AssetIDs...),
		BuyerEmail:  signup.BuyerEmail,
		OrderID:     signup.OrderID,
		MessageID:   messageID,
	}
	f.mu.Unlock()
	return token, messageID, nil
}

func (f *SignupFlow) Verify(token string) (MarketplaceState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	state, ok := f.pending[token]
	if !ok {
		return MarketplaceState{}, ErrUnknownVerification
	}
	state.EmailVerified = true
	state.BuyerUpdatesEnabled = true
	state.OrderHandoffReady = len(state.AssetIDs) > 0 && state.OrderID != ""
	f.pending[token] = state
	return state, nil
}

func newToken() (string, error) {
	value := make([]byte, 24)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
