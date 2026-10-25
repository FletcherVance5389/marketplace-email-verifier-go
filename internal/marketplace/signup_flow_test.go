package marketplace

import (
	"context"
	"testing"
)

type recordingSender struct {
	mail VerificationEmail
}

func (s *recordingSender) SendVerification(_ context.Context, mail VerificationEmail) (string, error) {
	s.mail = mail
	return "msg_test_42", nil
}

func TestVerificationControlsOrderHandoff(t *testing.T) {
	tests := []struct {
		name             string
		assets           []string
		wantHandoffReady bool
	}{
		{name: "seller has an asset to hand off", assets: []string{"asset-photo-set"}, wantHandoffReady: true},
		{name: "seller has no asset to hand off", assets: nil, wantHandoffReady: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sender := &recordingSender{}
			flow := NewSignupFlow(sender, "http://localhost:8080")
			token, messageID, err := flow.Start(context.Background(), Signup{
				SellerEmail: "seller@example.com",
				SellerName:  "Mina",
				AssetIDs:    test.assets,
				BuyerEmail:  "buyer@example.com",
				OrderID:     "order-42",
			})
			if err != nil {
				t.Fatal(err)
			}
			if messageID != "msg_test_42" || sender.mail.IdempotencyKey != "marketplace-signup-order-42" {
				t.Fatalf("unexpected send boundary: message=%q key=%q", messageID, sender.mail.IdempotencyKey)
			}

			state, err := flow.Verify(token)
			if err != nil {
				t.Fatal(err)
			}
			if !state.EmailVerified || !state.BuyerUpdatesEnabled {
				t.Fatalf("verification did not enable buyer updates: %+v", state)
			}
			if state.OrderHandoffReady != test.wantHandoffReady {
				t.Fatalf("handoff ready = %v, want %v", state.OrderHandoffReady, test.wantHandoffReady)
			}
		})
	}
}
