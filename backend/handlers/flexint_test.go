package handlers

import (
	"encoding/json"
	"testing"
)

func TestFlexIntUnmarshal(t *testing.T) {
	var req struct {
		SortOrder *FlexInt `json:"sort_order"`
	}
	// 1) 数字
	if err := json.Unmarshal([]byte(`{"sort_order": 5}`), &req); err != nil {
		t.Fatalf("number unmarshal error: %v", err)
	}
	if req.SortOrder == nil || int(*req.SortOrder) != 5 {
		t.Fatalf("number: got %+v, want 5", req.SortOrder)
	}
	// 2) 数字字符串
	if err := json.Unmarshal([]byte(`{"sort_order": "3"}`), &req); err != nil {
		t.Fatalf("string unmarshal error: %v", err)
	}
	if req.SortOrder == nil || int(*req.SortOrder) != 3 {
		t.Fatalf("string: got %+v, want 3", req.SortOrder)
	}
	// 3) 空字符串 -> 0
	if err := json.Unmarshal([]byte(`{"sort_order": ""}`), &req); err != nil {
		t.Fatalf("empty string unmarshal error: %v", err)
	}
	if req.SortOrder == nil || int(*req.SortOrder) != 0 {
		t.Fatalf("empty string: got %+v, want 0", req.SortOrder)
	}
	// 4) 缺字段 -> nil
	req.SortOrder = nil
	if err := json.Unmarshal([]byte(`{}`), &req); err != nil {
		t.Fatalf("missing unmarshal error: %v", err)
	}
	if req.SortOrder != nil {
		t.Fatalf("missing: got %+v, want nil", req.SortOrder)
	}
}
