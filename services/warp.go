package services

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/crypto/curve25519"
)

type WarpAccount struct {
	PrivateKey     string `json:"private_key"`
	PublicKey      string `json:"public_key"`
	LocalAddressV4 string `json:"local_address_v4"`
	LocalAddressV6 string `json:"local_address_v6"`
	Reserved       []int  `json:"reserved"`
}

func RegisterWarp() (*WarpAccount, error) {
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return nil, err
	}
	var pub [32]byte
	curve25519.ScalarBaseMult(&pub, &priv)

	privBase64 := base64.StdEncoding.EncodeToString(priv[:])
	pubBase64 := base64.StdEncoding.EncodeToString(pub[:])

	reqData := map[string]interface{}{
		"key":           pubBase64,
		"install_id":    "",
		"fcm_token":     "",
		"tos":           time.Now().Format(time.RFC3339Nano),
		"model":         "PC",
		"serial_number": "",
		"locale":        "en_US",
	}

	body, _ := json.Marshal(reqData)

	req, err := http.NewRequest("POST", "https://api.cloudflareclient.com/v0a884/reg", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "okhttp/3.12.1")
	req.Header.Set("CF-Client-Version", "a-6.11-2223")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	resBytes, _ := io.ReadAll(resp.Body)
	var resData struct {
		Config struct {
			Peers []struct {
				Endpoint struct {
					V4 string `json:"v4"`
					V6 string `json:"v6"`
				} `json:"endpoint"`
				PublicKey string `json:"public_key"`
			} `json:"peers"`
			Interface struct {
				Addresses struct {
					V4 string `json:"v4"`
					V6 string `json:"v6"`
				} `json:"addresses"`
			} `json:"interface"`
			ClientId string `json:"client_id"`
		} `json:"config"`
	}
	if err := json.Unmarshal(resBytes, &resData); err != nil {
		return nil, err
	}

	decodedClientId, err := base64.StdEncoding.DecodeString(resData.Config.ClientId)
	var reserved []int
	if err == nil && len(decodedClientId) >= 3 {
		reserved = []int{int(decodedClientId[0]), int(decodedClientId[1]), int(decodedClientId[2])}
	} else {
		reserved = []int{0, 0, 0}
	}

	return &WarpAccount{
		PrivateKey:     privBase64,
		PublicKey:      pubBase64,
		LocalAddressV4: resData.Config.Interface.Addresses.V4 + "/32",
		LocalAddressV6: resData.Config.Interface.Addresses.V6 + "/128",
		Reserved:       reserved,
	}, nil
}
