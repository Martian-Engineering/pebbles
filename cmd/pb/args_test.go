package main

import (
	"reflect"
	"testing"
)

func TestReorderFlagsMovesValueFlags(t *testing.T) {
	args := []string{"pb-1", "pb-2", "--type", "parent-child"}
	got := reorderFlags(args, map[string]bool{"--type": true})
	want := []string{"--type", "parent-child", "pb-1", "pb-2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected reorder: %v", got)
	}
}

func TestReorderFlagsMovesBoolFlags(t *testing.T) {
	args := []string{"pb", "--full"}
	got := reorderFlags(args, map[string]bool{})
	want := []string{"--full", "pb"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected reorder: %v", got)
	}
}

func TestReorderFlagsKeepsEquals(t *testing.T) {
	args := []string{"pb-1", "--type=parent-child"}
	got := reorderFlags(args, map[string]bool{"--type": true})
	want := []string{"--type=parent-child", "pb-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected reorder: %v", got)
	}
}
