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
const ddocVersion = 38
const designDoc = `
{
    "spatialInfos": [],
    "viewInfos": [
        {
            "map": "function (doc, meta) {\n  if (doc.type === \"bug\") {\n    emit([doc.status, doc.modified_at], null);\n  }\n}",
            "name": "aging",
            "removeLink": "#removeView=cbugg%2F_design%252Fcbugg%2F_view%2Faging",
            "viewLink": "#showView=cbugg%2F_design%252Fcbugg%2F_view%2Faging"
        },
        {
            "map": "function (doc, meta) {\n  if (doc.type === \"attachment\") {\n    emit([doc.bugId, doc.created_at], {url: doc.url,\n                                       type: doc.content_type,\n                                       user: doc.user,\n                                       size: doc.size});\n  }\n}",
            "name": "attachments",
            "removeLink": "#removeView=cbugg%2F_design%252Fcbugg%2F_view%2Fattachments",
            "viewLink": "#showView=cbugg%2F_design%252Fcbugg%2F_view%2Fattachments"
        },
        {
            "map": "function (doc, meta) {\n  if (doc.type === 'bughistory' || doc.type === 'bug') {\n    emit([doc.id, doc.modified_at], {\"type\": doc.modify_type || \"created\",\n                                     \"by\": doc.modified_by});\n  }\n}",
            "name": "bug_history",
            "removeLink": "#removeView=cbugg%2F_design%252Fcbugg%2F_view%2Fbug_history",
            "viewLink": "#showView=cbugg%2F_design%252Fcbugg%2F_view%2Fbug_history"
        },
        {
            "map": "function (doc, meta) {\n  if (doc.type === 'bug') {\n    emit([doc.status, doc.created_at], {title: doc.title,\n                                        owner: doc.owner,\n                                        status: doc.status,\n                                        tags:doc.tags});\n  }\n}",
            "name": "by_state",
            "reduce": "_count",
            "removeLink": "#removeView=cbugg%2F_design%252Fcbugg%2F_view%2Fby_state",
            "viewLink": "#showView=cbugg%2F_design%252Fcbugg%2F_view%2Fby_state"
        },
        {
            "map": "function (doc, meta) {\n  if ((doc.type === 'bughistory' || doc.type === \"bug\") && doc.modify_type) {\n    var ob = {actor: doc.modified_by,\n              action: \"changed \" + doc.modify_type + ' of',\n              bugid: doc.id};\n    if (doc.modify_type === 'created') {\n      ob.action = 'created';\n    }\n    emit(doc.modified_at, ob);\n  } else if (doc.type === 'comment') {\n    emit(doc.created_at, {actor: doc.user, action: \"commented on\", bugid: doc.bugId});\n  }\n}",
            "name": "changes",
            "removeLink": "#removeView=cbugg%2F_design%252Fcbugg%2F_view%2Fchanges",
            "viewLink": "#showView=cbugg%2F_design%252Fcbugg%2F_view%2Fchanges"
        },
        {
            "map": "function (doc, meta) {\n  if (doc.type === \"comment\" || doc.type === \"ping\") {\n    emit([doc.bugId, doc.created_at], doc.type);\n  }\n}",
            "name": "comments",
            "removeLink": "#removeView=cbugg%2F_design%252Fcbugg%2F_view%2Fcomments",
            "viewLink": "#showView=cbugg%2F_design%252Fcbugg%2F_view%2Fcomments"
        },
        {
            "map": "function (doc, meta) {\n  if (doc.type === 'bug' && doc.owner) {\n    emit([doc.owner, doc.status, doc.created_at], {title: doc.title,\n                                                   owner: doc.owner,\n                                                   status: doc.status,\n                                                   tags: doc.tags});\n  }\n}",
            "name": "owners",
            "reduce": "_count",
            "removeLink": "#removeView=cbugg%2F_design%252Fcbugg%2F_view%2Fowners",
            "viewLink": "#showView=cbugg%2F_design%252Fcbugg%2F_view%2Fowners"
        },
        {
            "map": "function (doc, meta) {\n  if (doc.type === \"reminder\") {\n    emit(doc.when, null);\n  }\n}",
            "name": "reminders",
            "removeLink": "#removeView=cbugg%2F_design%252Fcbugg%2F_view%2Freminders",
            "viewLink": "#showView=cbugg%2F_design%252Fcbugg%2F_view%2Freminders"
        },
        {
            "map": "function (doc, meta) {\n  if (doc.type === 'bug' && doc.tags) {\n    for (var i = 0; i < doc.tags.length; i++) {\n      emit([doc.tags[i], doc.status], 1);\n    }\n  } else if(doc.type === 'tag') {\n    emit([doc.name, \"inbox\"], 0);\n  }\n}",
            "name": "tags",
            "reduce": "_sum",
            "removeLink": "#removeView=cbugg%2F_design%252Fcbugg%2F_view%2Ftags",
            "viewLink": "#showView=cbugg%2F_design%252Fcbugg%2F_view%2Ftags"
        },
        {
            "map": "function (doc, meta) {\n  if (doc.type === 'bug') {\n    if (doc.creator) {\n      emit(doc.creator, null);\n    } else if(doc.modified_by && doc.modified_by != doc.creator) {\n      emit(doc.modified_by, null);\n    }\n  } else if(doc.type === \"user\") {\n    emit(doc.id, null);\n  } else if(doc.type === \"ping\") {\n    emit(doc.from, null);\n    emit(doc.to, null);\n  }\n}",
            "name": "users",
            "reduce": "_count",
            "removeLink": "#removeView=cbugg%2F_design%252Fcbugg%2F_view%2Fusers",
            "viewLink": "#showView=cbugg%2F_design%252Fcbugg%2F_view%2Fusers"
        }
    ],
    "views": {
        "aging": {
            "map": "function (doc, meta) {\n  if (doc.type === \"bug\") {\n    emit([doc.status, doc.modified_at], null);\n  }\n}"
        },
        "attachments": {
            "map": "function (doc, meta) {\n  if (doc.type === \"attachment\") {\n    emit([doc.bugId, doc.created_at], {url: doc.url,\n                                       type: doc.content_type,\n                                       user: doc.user,\n                                       size: doc.size});\n  }\n}"
        },
        "bug_history": {
            "map": "function (doc, meta) {\n  if (doc.type === 'bughistory' || doc.type === 'bug') {\n    emit([doc.id, doc.modified_at], {\"type\": doc.modify_type || \"created\",\n                                     \"by\": doc.modified_by});\n  }\n}"
        },
        "by_state": {
            "map": "function (doc, meta) {\n  if (doc.type === 'bug') {\n    emit([doc.status, doc.created_at], {title: doc.title,\n                                        owner: doc.owner,\n                                        status: doc.status,\n                                        tags:doc.tags});\n  }\n}",
            "reduce": "_count"
        },
        "changes": {
            "map": "function (doc, meta) {\n  if ((doc.type === 'bughistory' || doc.type === \"bug\") && doc.modify_type) {\n    var ob = {actor: doc.modified_by,\n              action: \"changed \" + doc.modify_type + ' of',\n              bugid: doc.id,\n              type: doc.type};\n    if (doc.modify_type === 'created') {\n      ob.action = 'created';\n    }\n    emit(doc.modified_at, ob);\n  } else if (doc.type === 'comment') {\n    emit(doc.created_at, {actor: doc.user, action: \"commented on\", type: doc.type, bugid: doc.bugId});\n  }\n}"
        },
        "comments": {
            "map": "function (doc, meta) {\n  if (doc.type === \"comment\" || doc.type === \"ping\") {\n    emit([doc.bugId, doc.created_at], doc.type);\n  }\n}"
        },
        "owners": {
            "map": "function (doc, meta) {\n  if (doc.type === 'bug' && doc.owner) {\n    emit([doc.owner, doc.status, doc.created_at], {title: doc.title,\n                                                   owner: doc.owner,\n                                                   status: doc.status,\n                                                   tags: doc.tags,\n                                                   mod: doc.modified_at});\n  }\n}",
            "reduce": "_count"
        },
        "reminders": {
            "map": "function (doc, meta) {\n  if (doc.type === \"reminder\") {\n    emit(doc.when, null);\n  }\n}"
        },
        "special_users": {
            "map": "function (doc, meta) {\n  if (doc.type === 'user') {\n    if (doc.admin) {\n      emit(\"admin\", doc.id);\n    }\n    if (doc.internal) {\n      emit(\"internal\", doc.id);\n    }\n  }\n}",
            "reduce": "_count"
        },
        "tag_info": {
            "map": "function (doc, meta) {\n  if (doc.type === 'tag') {\n    emit(doc.name, null);\n  }\n}"
        },
        "tags": {
            "map": "function (doc, meta) {\n  if (doc.type === 'bug' && doc.tags) {\n    for (var i = 0; i < doc.tags.length; i++) {\n      emit([doc.tags[i], doc.status], 1);\n    }\n  } else if(doc.type === 'tag') {\n    emit([doc.name, \"inbox\"], 0);\n  }\n}",
            "reduce": "_sum"
        },
        "users": {
            "map": "function (doc, meta) {\n  if (doc.type === 'bug') {\n    if (doc.creator) {\n      emit(doc.creator, null);\n    } else if(doc.modified_by && doc.modified_by != doc.creator) {\n      emit(doc.modified_by, null);\n    }\n  } else if(doc.type === \"user\") {\n    emit(doc.id, null);\n  } else if(doc.type === \"ping\") {\n    emit(doc.from, null);\n    emit(doc.to, null);\n  }\n}",
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
