package main

import (
	"testing"
)

func TestBugVisibility(t *testing.T) {
	internalUser := User{Id: "internal user", Internal: true}
	externalUser := User{Id: "external user"}
	specialUser := User{Id: "special user"}

	privateBug := Bug{Id: "private bug", Private: true,
		AlsoVisibleTo: []string{specialUser.Id}}
	publicBug := Bug{Id: "public bug"}

	privateAPIBug := APIBug{Id: "private api bug", Private: true,
		AlsoVisibleTo: []string{specialUser.Id}}
	publicAPIBug := APIBug{Id: "public api bug"}

	tests := []struct {
		u      User
		ob     interface{}
		result bool
	}{
		{internalUser, privateBug, true},
		{internalUser, publicBug, true},
		{internalUser, privateAPIBug, true},
		{internalUser, publicAPIBug, true},
		{externalUser, privateBug, false},
		{externalUser, publicBug, true},
		{externalUser, privateAPIBug, false},
		{externalUser, publicAPIBug, true},
		{specialUser, privateBug, true},
		{specialUser, publicBug, true},
		{specialUser, privateAPIBug, true},
		{specialUser, publicAPIBug, true},
	}

	for _, x := range tests {
		if isVisible(x.ob, x.u) != x.result {
			t.Errorf("isVisible(%+v, %+v), expected %v, got %v",
				x.ob, x.u, x.result, isVisible(x.ob, x.u))
		}
	}
}

func TestCommentVisibility(t *testing.T) {
	internalUser := User{Id: "internal user", Internal: true}
	externalUser := User{Id: "external user"}

	privateComment := Comment{Id: "private comment", Private: true}
	publicComment := Comment{Id: "public comment"}

	privateAPIComment := APIComment{Id: "private api comment", Private: true}
	publicAPIComment := APIComment{Id: "public api comment"}

	tests := []struct {
		u      User
		ob     interface{}
		result bool
	}{
		{internalUser, privateComment, true},
		{internalUser, publicComment, true},
		{internalUser, privateAPIComment, true},
		{internalUser, publicAPIComment, true},
		{externalUser, privateComment, false},
		{externalUser, publicComment, true},
		{externalUser, privateAPIComment, false},
		{externalUser, publicAPIComment, true},
	}

	for _, x := range tests {
		if isVisible(x.ob, x.u) != x.result {
			t.Errorf("isVisible(%+v, %+v), expected %v, got %v",
				x.ob, x.u, x.result, isVisible(x.ob, x.u))
		}
	}
}
