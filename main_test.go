package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSolanaAccountInfo(t *testing.T) {
	originalURL := getSolanaRPCURL()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var response string
		if r.Body != nil {
			defer r.Body.Close()
			var req RPCRequest
			json.NewDecoder(r.Body).Decode(&req)
			switch req.Method {
			case "getAccountInfo":
				response = `{"jsonrpc":"2.0","result":{"context":{"slot":1234},"value":{"data":["","base64"],"executable":false,"lamports":1000000000,"owner":"11111111111111111111111111111111","rentEpoch":0}},"id":1}`
			case "getProgramAccounts":
				response = `{"jsonrpc":"2.0","result":[{"account":{"data":{"parsed":{"info":{"meta":{"authorized":{"staker":"Stake11111111111111111111111111111111111111","withdrawer":"Stake11111111111111111111111111111111111111"},"lockup":{"custodian":"11111111111111111111111111111111","epoch":0,"unixTimestamp":0},"rentExemptReserve":"2282880"},"stake":{"creditsObserved":1234,"delegation":{"activationEpoch":"123","deactivationEpoch":"18446744073709551615","stake":"500000000","voter":"Vote111111111111111111111111111111111111111"}}},"type":"delegated"},"program":"stake","space":200},"executable":false,"lamports":500000000,"owner":"Stake11111111111111111111111111111111111111","rentEpoch":0},"pubkey":"TESTPUBKEY"}],"id":1}`
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	}))
	defer server.Close()

	setSolanaRPCURL(server.URL)

	ctx := context.Background()
	address := "TestAddress123"

	info, err := getSolanaAccountInfo(ctx, address)
	if err != nil {
		t.Fatalf("getSolanaAccountInfo failed: %v", err)
	}

	expectedBalance := 1.0
	if info.Balance != expectedBalance {
		t.Errorf("Expected balance %f, got %f", expectedBalance, info.Balance)
	}

	expectedStakedBalance := 0.5
	if info.StakedBalance != expectedStakedBalance {
		t.Errorf("Expected staked balance %f, got %f", expectedStakedBalance, info.StakedBalance)
	}

	setSolanaRPCURL(originalURL)
}
