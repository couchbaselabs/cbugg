package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/couchbaselabs/go-couchbase"
)

type viewMarker struct {
	Version   int       `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
}

const ddocKey = "/@cbuggddocVersion"
const ddocVersion = 1
const designDoc = `
{
    "views": {
        "by_state": {
            "map": "function (doc, meta) {\n  if (doc.type === 'bug') {\n    emit([doc.status, doc.created_at], doc.title);\n  }\n}",
            "reduce": "_count"
        }
    }
}
`

func dbConnect(serv, bucket string) (*couchbase.Bucket, error) {

	log.Printf("Connecting to couchbase bucket %v at %v",
		bucket, serv)
	rv, err := couchbase.GetBucket(serv, "default", bucket)
	if err != nil {
		return nil, err
	}

	marker := viewMarker{}
	err = rv.Get(ddocKey, &marker)
	if err != nil {
		log.Printf("Error checking view version: %v", err)
	}
	if marker.Version < ddocVersion {
		log.Printf("Installing new version of views (old version=%v)",
			marker.Version)
		doc := json.RawMessage([]byte(designDoc))
		err = rv.PutDDoc("cbfs", &doc)
		if err != nil {
			return nil, err
		}
		marker.Version = ddocVersion
		marker.Timestamp = time.Now().UTC()
		marker.Type = "ddocmarker"

		rv.Set(ddocKey, 0, &marker)
	}

	return rv, nil
}
