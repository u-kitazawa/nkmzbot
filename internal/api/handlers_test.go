package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleWebInterface(t *testing.T) {
	api := &API{}
	
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	
	api.handleWebInterface(w, req)
	
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.StatusCode)
	}
	
	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type text/html; charset=utf-8, got %v", contentType)
	}
	
	// Check that the response contains key elements
	body := w.Body.String()
	expectedStrings := []string{
		"<!DOCTYPE html>",
		"nkmzbot",
		"コマンド一覧",
		"loadCommands",
	}
	
	for _, expected := range expectedStrings {
		if !strings.Contains(body, expected) {
			t.Errorf("Expected response to contain '%s'", expected)
		}
	}
}

func TestHandleWebInterfaceWithGuildId(t *testing.T) {
	api := &API{}
	
	req := httptest.NewRequest("GET", "/guilds/123456789", nil)
	w := httptest.NewRecorder()
	
	api.handleWebInterface(w, req)
	
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.StatusCode)
	}
}
