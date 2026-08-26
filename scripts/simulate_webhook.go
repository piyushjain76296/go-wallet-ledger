package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
)

func main() {
	secret := "super_secret_key_change_me_in_prod" // from docker-compose.yml
	payload := []byte(`{
		"event_id": "evt_001",
		"event_type": "payment.succeeded",
		"reference_id": "ext_9ULXnvqaGSPG",
		"status": "SUCCEEDED"
	}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))

	req, _ := http.NewRequest("POST", "http://localhost:8080/api/v1/webhooks/payment-provider", bytes.NewBuffer(payload))
	req.Header.Set("X-Webhook-Signature", sig)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()
	fmt.Println("Webhook status:", resp.Status)
}
