package main

import (
	"encoding/json"
	"testing"
)

func TestGetExactInt64ParamRejectsNonIntegralNumber(t *testing.T) {
	_, err := getExactInt64Param(map[string]interface{}{"id": json.Number("5.5")}, "id")
	if err == nil {
		t.Fatal("expected non-integral number to be rejected")
	}
}

func TestGetExactInt64ParamPreservesInteger(t *testing.T) {
	got, err := getExactInt64Param(map[string]interface{}{"id": json.Number("9007199254740993")}, "id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 9007199254740993 {
		t.Fatalf("got %d", got)
	}
}
