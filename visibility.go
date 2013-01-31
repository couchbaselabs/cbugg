package main

type PartiallyVisible interface {
	IsVisibleTo(u User) bool
}

func isVisible(ob interface{}, u User) bool {
	if pv, ok := ob.(PartiallyVisible); ok {
		return pv.IsVisibleTo(u)
	}
	return true
}

func filterUnprivelegedEmails(ob interface{}, emails []string) []string {
	rv := []string{}

	for _, e := range emails {
		u, _ := getUser(e)
		if isVisible(ob, u) {
			rv = append(rv, e)
		}
	}

	return rv
}
