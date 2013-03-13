package main

import (
	"encoding/json"
	"flag"
	"log"

	"github.com/couchbaselabs/go-couchbase"
)

var db *couchbase.Bucket

type User struct {
	Id        string                 `json:"id"`
	Type      string                 `json:"type"`
	Admin     bool                   `json:"admin"`
	AuthToken string                 `json:"auth_token,omitmepty"`
	Internal  bool                   `json:"internal"`
	Prefs     map[string]interface{} `json:"prefs"`
}

func updateUser(email string, isAdmin, isInternal bool) {
	key := "u-" + email

	var user User
	err := db.Update(key, 0, func(current []byte) ([]byte, error) {
		if len(current) > 0 {
			err := json.Unmarshal(current, &user)
			if err != nil {
				return nil, err
			}
		}

		// Common fields
		user.Id = email
		user.Type = "user"
		user.Admin = isAdmin
		user.Internal = isInternal

		return json.Marshal(user)
	})

	if err != nil {
		log.Fatalf("Error updating user: %v", err)
	}
	log.Printf("Updated user: %+v", user)
}

func main() {
	dburl := flag.String("couchbase", "http://localhost:8091/",
		"URL to couchbase")
	bucketName := flag.String("bucket", "cbugg", "cbugg bucket")
	isAdmin := flag.Bool("admin", false, "should this user be an admin?")
	isInternal := flag.Bool("internal", false,
		"should this user be internal?")

	flag.Parse()

	email := flag.Arg(0)
	if email == "" {
		log.Fatalf("Need an email address")
	}

	var err error
	db, err = couchbase.GetBucket(*dburl, "default", *bucketName)
	if err != nil {
		log.Fatalf("Error connecting to couchbase: %v", err)
	}

	updateUser(email, *isAdmin, *isInternal)
}
