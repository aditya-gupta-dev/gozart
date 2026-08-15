package main

import (
	"gozart/utils"
	"testing"
)

func TestFormatSize(t *testing.T) {
	var param float64 = 1200000000
	got := utils.FormatSize(param)
	want := "1.12 GB"

	if got != want {
		t.Errorf("FormatSize(%f) = %s but wanted %s", param, got, want)
	}
}
