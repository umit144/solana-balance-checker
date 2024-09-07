package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
)

var (
	solanaRPCURLMutex sync.RWMutex
	solanaRPCURL      = "https://api.mainnet-beta.solana.com"
)

const (
	timeout = 10 * time.Second
)

type RPCRequest struct {
	JsonRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type AccountInfo struct {
	Balance       float64
	StakedBalance float64
}

func main() {
	address := getAddressFromUser()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	accountInfo, err := getSolanaAccountInfo(ctx, address)
	if err != nil {
		fmt.Printf("Error getting account info: %v\n", err)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.Debug)

	fmt.Fprintln(w, "Category\tValue")
	fmt.Fprintln(w, "--------\t-----")
	fmt.Fprintf(w, "Address\t%s\n", address)
	fmt.Fprintf(w, "Current Balance\t%.9f SOL\n", accountInfo.Balance)
	fmt.Fprintf(w, "Staked Balance\t%.9f SOL\n", accountInfo.StakedBalance)
	fmt.Fprintf(w, "Total Balance\t%.9f SOL\n", accountInfo.Balance+accountInfo.StakedBalance)

	w.Flush()
}

func getAddressFromUser() string {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Please enter a Solana wallet address: ")
		address, _ := reader.ReadString('\n')
		address = strings.TrimSpace(address)

		if len(address) == 44 || len(address) == 43 {
			return address
		}
		fmt.Println("Invalid address length. Please try again.")
	}
}

func getSolanaRPCURL() string {
	solanaRPCURLMutex.RLock()
	defer solanaRPCURLMutex.RUnlock()
	return solanaRPCURL
}

func setSolanaRPCURL(url string) {
	solanaRPCURLMutex.Lock()
	solanaRPCURL = url
	solanaRPCURLMutex.Unlock()
}

func getSolanaAccountInfo(ctx context.Context, address string) (*AccountInfo, error) {
	accountInfo, err := sendRPCRequest(ctx, "getAccountInfo", []interface{}{
		address,
		map[string]interface{}{"encoding": "jsonParsed"},
	})
	if err != nil {
		return nil, err
	}

	var accountData struct {
		Value struct {
			Lamports float64 `json:"lamports"`
		} `json:"value"`
	}
	if err := json.Unmarshal(accountInfo, &accountData); err != nil {
		return nil, fmt.Errorf("error parsing account info: %w", err)
	}

	stakeAccounts, err := sendRPCRequest(ctx, "getProgramAccounts", []interface{}{
		"Stake11111111111111111111111111111111111111",
		map[string]interface{}{
			"encoding": "jsonParsed",
			"filters": []map[string]interface{}{
				{"memcmp": map[string]interface{}{"offset": 12, "bytes": address}},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	balance := accountData.Value.Lamports / 1e9
	stakedBalance, err := calculateTotalStake(stakeAccounts)
	if err != nil {
		return nil, err
	}

	return &AccountInfo{
		Balance:       balance,
		StakedBalance: stakedBalance,
	}, nil
}

func sendRPCRequest(ctx context.Context, method string, params []interface{}) (json.RawMessage, error) {
	requestBody, err := json.Marshal(RPCRequest{
		JsonRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return nil, fmt.Errorf("JSON marshaling error: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, getSolanaRPCURL(), bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("request creation error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("response reading error: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status code: %d, response: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("JSON parsing error: %w, response: %s", err, string(body))
	}

	return response.Result, nil
}

func calculateTotalStake(rawStakeAccounts json.RawMessage) (float64, error) {
	var stakeAccounts []struct {
		Account struct {
			Data struct {
				Parsed struct {
					Type string `json:"type"`
					Info struct {
						Stake struct {
							Delegation struct {
								Stake string `json:"stake"`
							} `json:"delegation"`
						} `json:"stake"`
					} `json:"info"`
				} `json:"parsed"`
			} `json:"data"`
		} `json:"account"`
	}

	if err := json.Unmarshal(rawStakeAccounts, &stakeAccounts); err != nil {
		return 0, fmt.Errorf("error parsing stake accounts: %w", err)
	}

	var totalStake uint64
	for _, account := range stakeAccounts {
		if account.Account.Data.Parsed.Type == "delegated" {
			stake, err := strconv.ParseUint(account.Account.Data.Parsed.Info.Stake.Delegation.Stake, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("error parsing stake value: %w", err)
			}
			totalStake += stake
		}
	}

	return float64(totalStake) / 1e9, nil
}
