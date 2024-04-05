package main

import (
	"bytes"
	"encoding/json"
	logger "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"time"
)

// Struct to capture the seal status response from Vault
type SealStatus struct {
	Sealed bool `json:"sealed"`
}

// Struct for unseal request payload
type UnsealRequest struct {
	Key string `json:"key"`
}

// Function to check and unseal a single Vault server
func checkAndUnsealVault(server string, unsealKeys []string, logLevel string) {

	resp, err := http.Get(server + "/v1/sys/seal-status")
	if err != nil {
		logger.Error("Error fetching seal status:", err)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Error reading response body:", err)
		return
	}

	var status SealStatus
	err = json.Unmarshal(body, &status)
	if err != nil {
		logger.Error("Error unmarshaling JSON:", err)
		return
	}

	if status.Sealed {
		logger.Info(server, "is sealed. Attempting to unseal...")
		for _, key := range unsealKeys {
			jsonData := UnsealRequest{Key: key}
			jsonValue, _ := json.Marshal(jsonData)

			unsealResp, err := http.Post(server+"/v1/sys/unseal", "application/json", bytes.NewBuffer(jsonValue))
			if err != nil {

				logger.Error("Error posting unseal request:", err)
				return
			}
			unsealResp.Body.Close()

			// Check if unseal was successful by re-checking the seal status
			checkResp, err := http.Get(server + "/v1/sys/seal-status")
			if err != nil {
				logger.Error("Error re-checking seal status:", err)
				return
			}
			defer checkResp.Body.Close()

			body, _ := ioutil.ReadAll(checkResp.Body)
			json.Unmarshal(body, &status)

			if !status.Sealed {
				logger.Info(server, " is now unsealed.")
				break
			}
		}
	} else {
		logger.Info(server, " is already unsealed.")
	}
}

func monitorAndUnsealVaults(servers []string, unsealKeys []string, probeInterval int, logLevel string) {
	for {
		for _, server := range servers {
			go checkAndUnsealVault(server, unsealKeys, logLevel)
		}

		time.Sleep(time.Duration(probeInterval) * time.Second)
	}
}
