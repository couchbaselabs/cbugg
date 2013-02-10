package main

import (
	"reflect"
	"testing"
)

func TestChangeRing(t *testing.T) {
	r := newChangeRing(3)

	res := r.Slice()
	if len(res) > 0 {
		t.Fatalf("Error slicing empty thing, got %v:", res)
	}

	r.Add(1)
	res = r.Slice()
	if !reflect.DeepEqual(res, []interface{}{1}) {
		t.Fatalf("Expected [1], got %v", res)
	}

	r.Add(2)
	res = r.Slice()
	if !reflect.DeepEqual(res, []interface{}{1, 2}) {
		t.Fatalf("Expected [1, 2], got %v", res)
	}

	r.Add(3)
	res = r.Slice()
	if !reflect.DeepEqual(res, []interface{}{1, 2, 3}) {
		t.Fatalf("Expected [1, 2, 3], got %v", res)
	}

	r.Add(4)
	res = r.Slice()
	if !reflect.DeepEqual(res, []interface{}{2, 3, 4}) {
		t.Fatalf("Expected [2, 3, 4], got %v", res)
	}
}
