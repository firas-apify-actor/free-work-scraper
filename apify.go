package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

// Client talks to the Apify platform, or to ./storage when run locally.
// ponytail: dataset/KV helpers only; charging arrives in #21.
type Client struct {
	runID, token, datasetID, kvID string
	dir                           string // local storage root
	seq                           int
}

func NewClient() *Client {
	c := &Client{
		runID:     os.Getenv("ACTOR_RUN_ID"),
		token:     os.Getenv("APIFY_TOKEN"),
		datasetID: os.Getenv("ACTOR_DEFAULT_DATASET_ID"),
		kvID:      os.Getenv("ACTOR_DEFAULT_KEY_VALUE_STORE_ID"),
		dir:       "storage",
	}
	return c
}

func (c *Client) local() bool { return c.runID == "" }

func (c *Client) api(method, path string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequest(method, "https://api.apify.com/v2"+path, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return b, resp.StatusCode, err
}

// storeID resolves a named key-value store ("" = run default), creating it if needed.
func (c *Client) storeID(name string) (string, error) {
	if name == "" {
		return c.kvID, nil
	}
	b, code, err := c.api("POST", "/key-value-stores?name="+url.QueryEscape(name), nil)
	if err != nil || code >= 300 {
		return "", fmt.Errorf("resolve store %q: %d %v %s", name, code, err, b)
	}
	var r struct{ Data struct{ ID string } }
	if err := json.Unmarshal(b, &r); err != nil {
		return "", err
	}
	return r.Data.ID, nil
}

// GetValue decodes the JSON record key of store (name "" = default) into v.
// Returns false if the record does not exist.
func (c *Client) GetValue(store, key string, v any) (bool, error) {
	var b []byte
	if c.local() {
		if store == "" {
			store = "default"
		}
		var err error
		b, err = os.ReadFile(filepath.Join(c.dir, "key_value_stores", store, key+".json"))
		if os.IsNotExist(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}
	} else {
		id, err := c.storeID(store)
		if err != nil {
			return false, err
		}
		var code int
		b, code, err = c.api("GET", "/key-value-stores/"+id+"/records/"+key, nil)
		if err != nil {
			return false, err
		}
		if code == 404 {
			return false, nil
		}
		if code >= 300 {
			return false, fmt.Errorf("get %s: %d %s", key, code, b)
		}
	}
	return true, json.Unmarshal(b, v)
}

func (c *Client) SetValue(store, key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if c.local() {
		if store == "" {
			store = "default"
		}
		p := filepath.Join(c.dir, "key_value_stores", store, key+".json")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		return os.WriteFile(p, b, 0o644)
	}
	id, err := c.storeID(store)
	if err != nil {
		return err
	}
	if out, code, err := c.api("PUT", "/key-value-stores/"+id+"/records/"+key, b); err != nil || code >= 300 {
		return fmt.Errorf("set %s: %d %v %s", key, code, err, out)
	}
	return nil
}

// Input decodes the INPUT record of the default store into v.
func (c *Client) Input(v any) error {
	_, err := c.GetValue("", "INPUT", v)
	return err
}

// PushData appends items to the default dataset.
func (c *Client) PushData(items ...any) error {
	if c.local() {
		dir := filepath.Join(c.dir, "datasets", "default")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if c.seq == 0 { // continue after files left by an earlier run
			old, _ := os.ReadDir(dir)
			c.seq = len(old)
		}
		for _, it := range items {
			c.seq++
			b, err := json.MarshalIndent(it, "", "  ")
			if err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%09d.json", c.seq)), b, 0o644); err != nil {
				return err
			}
		}
		return nil
	}
	b, err := json.Marshal(items)
	if err != nil {
		return err
	}
	if out, code, err := c.api("POST", "/datasets/"+c.datasetID+"/items", b); err != nil || code >= 300 {
		return fmt.Errorf("push: %d %v %s", code, err, out)
	}
	return nil
}
