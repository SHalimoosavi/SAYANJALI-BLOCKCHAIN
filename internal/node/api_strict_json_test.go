package node

import (
	"strings"
	"testing"
)

func TestDecodeStrictJSON(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "single object", body: `{"name":"syj"}`},
		{name: "trailing whitespace", body: "{\"name\":\"syj\"} \n\t"},
		{name: "unknown field", body: `{"name":"syj","unexpected":true}`, wantErr: true},
		{name: "second JSON value", body: `{"name":"syj"} {"name":"other"}`, wantErr: true},
		{name: "trailing garbage", body: `{"name":"syj"} trailing`, wantErr: true},
		{name: "malformed JSON", body: `{"name":`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got request
			err := decodeStrictJSON(strings.NewReader(tt.body), &got)
			if (err != nil) != tt.wantErr {
				t.Fatalf("decodeStrictJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got.Name != "syj" {
				t.Fatalf("decoded name = %q, want syj", got.Name)
			}
		})
	}
}
