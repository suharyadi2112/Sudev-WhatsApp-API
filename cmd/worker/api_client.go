package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type InstanceInfo struct {
	InstanceID  string `json:"instanceId"`
	PhoneNumber string `json:"phoneNumber"`
}

type SudevwaClient struct {
	BaseURL      string
	Username     string
	Password     string
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time

	// Caching instances to reduce API load
	mu                sync.RWMutex
	allInstancesCache []struct {
		InstanceID  string `json:"instanceId"`
		PhoneNumber string `json:"phoneNumber"`
		Used        bool   `json:"used"`
		Circle      string `json:"circle"`
		Status      string `json:"status"`
	}
	cacheExpiry time.Time
}

type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Instances    []struct {
			InstanceID  string `json:"instanceId"`
			PhoneNumber string `json:"phoneNumber"`
			Used        bool   `json:"used"`
			Circle      string `json:"circle"`
			Status      string `json:"status"`
		} `json:"instances"`
	} `json:"data"`
}

func NewSudevwaClient(baseURL, username, password string) *SudevwaClient {
	return &SudevwaClient{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
	}
}

func (c *SudevwaClient) EnsureAuth() error {
	if c.AccessToken == "" || time.Now().After(c.ExpiresAt) {
		if c.RefreshToken != "" {
			err := c.Refresh()
			if err == nil {
				return nil
			}
		}
		return c.Login()
	}
	return nil
}

func (c *SudevwaClient) Login() error {
	payload, _ := json.Marshal(map[string]string{
		"username": c.Username,
		"password": c.Password,
	})

	resp, err := http.Post(c.BaseURL+"/login", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var res APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}

	if !res.Success {
		return fmt.Errorf("login failed: %s", res.Message)
	}

	c.AccessToken = res.Data.AccessToken
	c.RefreshToken = res.Data.RefreshToken
	c.ExpiresAt = time.Now().Add(50 * time.Minute)

	return nil
}

func (c *SudevwaClient) Refresh() error {
	payload, _ := json.Marshal(map[string]string{
		"refresh_token": c.RefreshToken,
	})

	resp, err := http.Post(c.BaseURL+"/refresh", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var res APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}

	if !res.Success {
		return fmt.Errorf("refresh failed: %s", res.Message)
	}

	c.AccessToken = res.Data.AccessToken
	c.ExpiresAt = time.Now().Add(50 * time.Minute)

	return nil
}

func (c *SudevwaClient) GetInstances(circle string) ([]InstanceInfo, error) {
	c.mu.RLock()
	if c.allInstancesCache != nil && time.Now().Before(c.cacheExpiry) {
		// Use cache
		var instances []InstanceInfo
		for _, inst := range c.allInstancesCache {
			if inst.Used && inst.Circle == circle {
				instances = append(instances, InstanceInfo{
					InstanceID:  inst.InstanceID,
					PhoneNumber: inst.PhoneNumber,
				})
			}
		}
		c.mu.RUnlock()
		return instances, nil
	}
	c.mu.RUnlock()

	// Cache expired or empty, fetch from API
	if err := c.EnsureAuth(); err != nil {
		return nil, err
	}

	req, _ := http.NewRequest("GET", c.BaseURL+"/api/instances?all=true", nil)
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	// Update cache
	c.mu.Lock()
	c.allInstancesCache = res.Data.Instances
	c.cacheExpiry = time.Now().Add(1 * time.Minute) // Cache for 1 minute
	c.mu.Unlock()

	var instances []InstanceInfo
	for _, inst := range res.Data.Instances {
		// Filter by circle and used status (BUT NOT online status to prevent spam)
		if inst.Used && inst.Circle == circle {
			instances = append(instances, InstanceInfo{
				InstanceID:  inst.InstanceID,
				PhoneNumber: inst.PhoneNumber,
			})
		}
	}

	return instances, nil
}

func (c *SudevwaClient) SendMessage(instanceID, to, message string) (bool, string, error) {
	if err := c.EnsureAuth(); err != nil {
		return false, "", err
	}

	payload, _ := json.Marshal(map[string]string{
		"to":      to,
		"message": message,
	})

	url := fmt.Sprintf("%s/api/send/%s", c.BaseURL, instanceID)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var res APIResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return false, string(body), err
	}

	return res.Success, res.Message, nil
}

func (c *SudevwaClient) SendGroupMessage(instanceID, groupID, message string) (bool, string, error) {
	if err := c.EnsureAuth(); err != nil {
		return false, "", err
	}

	payload, _ := json.Marshal(map[string]string{
		"message":  message,
		"groupJid": groupID,
	})

	url := fmt.Sprintf("%s/api/send-group/%s", c.BaseURL, instanceID)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var res APIResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return false, string(body), err
	}

	return res.Success, res.Message, nil
}

func (c *SudevwaClient) SendMediaURL(instanceID, to, mediaURL, caption string) (bool, string, error) {
	if err := c.EnsureAuth(); err != nil {
		return false, "", err
	}

	payload, _ := json.Marshal(map[string]string{
		"to":       to,
		"mediaUrl": mediaURL,
		"caption":  caption,
	})

	url := fmt.Sprintf("%s/api/send/%s/media-url", c.BaseURL, instanceID)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var res APIResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return false, string(body), err
	}

	return res.Success, res.Message, nil
}

func (c *SudevwaClient) SendGroupMediaURL(instanceID, groupID, mediaURL, caption string) (bool, string, error) {
	if err := c.EnsureAuth(); err != nil {
		return false, "", err
	}

	payload, _ := json.Marshal(map[string]string{
		"groupJid": groupID,
		"mediaUrl": mediaURL,
		"message":  caption,
	})

	url := fmt.Sprintf("%s/api/send-group/%s/media-url", c.BaseURL, instanceID)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var res APIResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return false, string(body), err
	}

	return res.Success, res.Message, nil
}
