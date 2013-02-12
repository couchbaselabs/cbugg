package main

import (
	"reflect"
	"testing"
)

func TestChangeRing(t *testing.T) {
	r := newChangeRing(3)

	// Empty slice
	res := r.Slice()
	if len(res) > 0 {
		t.Fatalf("Error slicing empty thing, got %v:", res)
	}

	// One item
	r.Add(1)
	res = r.Slice()
	if !reflect.DeepEqual(res, []interface{}{1}) {
		t.Fatalf("Expected [1], got %v", res)
	}

	// two
	r.Add(2)
	res = r.Slice()
	if !reflect.DeepEqual(res, []interface{}{1, 2}) {
		t.Fatalf("Expected [1, 2], got %v", res)
	}

	// three
	r.Add(3)
	res = r.Slice()
	if !reflect.DeepEqual(res, []interface{}{1, 2, 3}) {
		t.Fatalf("Expected [1, 2, 3], got %v", res)
	}

	// And at four, we shift out the first one.
	r.Add(4)
	res = r.Slice()
	if !reflect.DeepEqual(res, []interface{}{2, 3, 4}) {
		t.Fatalf("Expected [2, 3, 4], got %v", res)
	}

	//
	// Test "latest" things to clamp down the results
	//

	// When we request more than we have.
	res = r.Latest(10)
	if !reflect.DeepEqual(res, []interface{}{2, 3, 4}) {
		t.Fatalf("Expected latest(1)=[2, 3, 4], got %v", res)
	}

	// When we request less than we have
	res = r.Latest(2)
	if !reflect.DeepEqual(res, []interface{}{3, 4}) {
		t.Fatalf("Expected latest(2)=[3, 4], got %v", res)
	}
}
