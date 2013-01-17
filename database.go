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
const ddocVersion = 14
const designDoc = `
{
    "views": {
        "attachments": {
            "map": "function (doc, meta) {\n  if (doc.type === \"attachment\") {\n    emit([doc.bugId, doc.created_at], {url: doc.url,\n                                       type: doc.content_type,\n                                       user: doc.user,\n                                       size: doc.size});\n  }\n}"
        },
        "bug_history": {
            "map": "function (doc, meta) {\n  if (doc.type === 'bughistory') {\n    emit([doc.id, doc.modified_at], {\"type\": doc.modify_type, \"by\": doc.modified_by});\n  }\n}"
        },
        "by_state": {
            "map": "function (doc, meta) {\n  if (doc.type === 'bug') {\n    emit([doc.status, doc.created_at], {title: doc.title, owner: doc.owner});\n  }\n}",
            "reduce": "_count"
        },
        "changes": {
            "map": "function (doc, meta) {\n  if (doc.type === 'bughistory') {\n    var ob = {actor: doc.modified_by,\n              action: \"changed \" + doc.modify_type + ' of',\n              bugid: doc.id};\n    \n    emit(doc.modified_at, ob);\n  } else if (doc.type === 'comment') {\n    emit(doc.created_at, {actor: doc.user, action: \"commented on\", bugid: doc.bugId});\n  } else if (doc.type === 'bug') {\n    emit(doc.created_at, {actor: doc.creator, action: \"created\", bugid: doc.id});\n  }\n}"
        },
        "comments": {
            "map": "function (doc, meta) {\n  if (doc.type === \"comment\") {\n    emit([doc.bugId, doc.created_at], null);\n  }\n}"
        },
        "owners": {
            "map": "function (doc, meta) {\n  if (doc.type === 'bug' && doc.owner) {\n    emit([doc.owner, doc.status, doc.created_at], {title: doc.title, owner: doc.owner});\n  }\n}",
            "reduce": "_count"
        },
        "tags": {
            "map": "function (doc, meta) {\n  if (doc.type === 'bug' && doc.tags) {\n    for (var i = 0; i < doc.tags.length; i++) {\n      emit(doc.tags[i], null);\n    }\n  }\n}",
            "reduce": "_count"
        },
        "users": {
            "map": "function (doc, meta) {\n  if (doc.type === 'bug' && doc.creator) {\n    emit(doc.creator, null);\n  }\n}",
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
		err = rv.PutDDoc("cbugg", &doc)
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
