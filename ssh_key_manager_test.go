package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMapPage(t *testing.T) {

	group1 := Group{Name: "group1", Keys: []string{"key1", "key2"}}
	group2 := Group{Name: "group2", Keys: []string{"key3", "", "key4"}}
	am := &AuthMap{Groups: []Group{group1, group2}}

	handler := authMapPage(am)

	if len(am.Groups[0].Keys) != 2 || len(am.Groups[1].Keys) != 3 {
		t.Error("Initial fault")
	}

	rr := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/authmap", nil)
	if err != nil {
		t.Fatal(err)
	}
	handler.ServeHTTP(rr, req)

	var resp AuthMap
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.Groups[0].Keys) != 2 {
		t.Error("Response group corrupted!")
	}
	if len(resp.Groups[1].Keys) != 2 {
		t.Error("Response group contains empty values!")
	}

	if len(am.Groups[0].Keys) != 2 || len(am.Groups[1].Keys) != 3 {
		t.Error("Initial authmap changed!")
	}
}
