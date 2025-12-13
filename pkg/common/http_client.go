package common

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Post sends a POST request to the specified URL with the given payload and headers.
// It returns the response body as a map[string]interface{} or an error.
func Post(url string, payload interface{}, headers map[string]string) (interface{}, error) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &result); err != nil {
			return string(body), nil
		}
	}

	return result, nil
}

// PostForm sends a POST request with x-www-form-urlencoded body
func PostForm(urlStr string, data url.Values) (interface{}, error) {
	resp, err := http.PostForm(urlStr, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &result); err != nil {
			return string(body), nil
		}
	}
	return result, nil
}

// PostXML sends a POST request with XML body
func PostXML(urlStr string, xmlData string, headers map[string]string) (interface{}, error) {
	req, err := http.NewRequest("POST", urlStr, strings.NewReader(xmlData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/xml")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Returns raw body string usually, or try to decode if needed?
	// Tigo might return XML or JSON.
	// For now return string or map if it happens to be JSON.
	var result interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &result); err == nil {
			return result, nil
		}
	}
	return string(body), nil
}

// Get sends a GET request to the specified URL with the given headers.
// It returns the response body as a map[string]interface{} or an error.
func Get(url string, headers map[string]string) (interface{}, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &result); err != nil {
			return string(body), nil
		}
	}

	return result, nil
}
