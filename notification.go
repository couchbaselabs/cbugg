package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dustin/gomemcached"
)

type bugChange struct {
	bugid     string
	actor     string
	fields    []string
	exception string
}

type bugPing struct {
	bug      Bug
	from, to string
}

type bugTagged struct {
	bugid, tag, actor string
}

var commentChan = make(chan Comment, 100)
var attachmentChan = make(chan Attachment, 100)
var bugChan = make(chan bugChange, 100)
var assignedChan = make(chan string, 100)
var pingChan = make(chan bugPing, 100)
var tagChan = make(chan bugTagged, 100)

var bugNotifyDelays map[string]chan bugChange
var bugNotifyDelayLock sync.Mutex

var bugDelay = flag.Duration("notificationDelay",
	time.Duration(10*time.Second),
	"bug change stabilization delay timer")

func init() {
	bugNotifyDelays = make(map[string]chan bugChange)

	go notificationLoop()
}

func notifyComment(c Comment) {
	commentChan <- c
}

func notifyAttachment(a Attachment) {
	attachmentChan <- a
}

func notifyBugPing(b Bug, from, to string) {
	pingChan <- bugPing{b, from, to}
}

func notifyTagAssigned(bugid, tag, actor string) {
	tagChan <- bugTagged{bugid, tag, actor}
}

func notifyBugChange(bugid, field, actor string) {
	bugChan <- bugChange{
		bugid:  bugid,
		actor:  actor,
		fields: []string{field},
	}
}

// Don't send an update to this user in the current batch.
func exceptBugChange(bugid, email string) {
	bugChan <- bugChange{bugid: bugid, exception: email}
}

func notifyBugAssignment(bugid, assigned string) {
	assignedChan <- bugid
	exceptBugChange(bugid, assigned)
}

func sendNotifications(tmplName string, subs []string,
	fields map[string]interface{}) {

	fields["BaseURL"] = *baseURL
	fields["MailFrom"] = *mailFrom
	fields["InReplyToDom"] = *replyToDom

	if *mailServer == "" || *mailFrom == "" {
		log.Printf("Email not configured, would have sent this:")
		fields["MailTo"] = "someone@example.com"
		templates.ExecuteTemplate(os.Stderr, tmplName, fields)

		return
	}

	bug, hasBug := fields["Bug"].(Bug)

	for _, to := range subs {
		buf := &bytes.Buffer{}

		if hasBug && bug.Private && !emailIsInternal(to) {
			log.Printf("Skipping private notification of %v to %v", bug, to)
			continue
		}

		fields["MailTo"] = to
		err := templates.ExecuteTemplate(buf, tmplName, fields)

		if err != nil {
			log.Printf("Error building mail body: %v", err)
			continue
		}

		err = sendEmail(to, buf.Bytes())
		if err != nil {
			log.Printf("Error sending email: %v", err)
		} else {
			log.Printf("Sent %v to %v", tmplName, to)
		}
	}
}

func sendAttachmentNotification(a Attachment) {
	b, err := getBug(a.BugId)
	if err != nil {
		log.Printf("Error getting bug %v for attachment notification: %v",
			a.BugId, err)
		return
	}

	to := removeFromList(b.Subscribers, a.User)

	sendNotifications("attach_notification", to,
		map[string]interface{}{
			"Att": a,
			"Bug": b,
		})
}
func sendCommentNotification(c Comment) {
	b, err := getBug(c.BugId)
	if err != nil {
		log.Printf("Error getting bug %v for comment notification: %v",
			c.BugId, err)
		return
	}

	subs := b.Subscribers
	if c.Private {
		subs = subs[:0]
		for _, e := range b.Subscribers {
			if emailIsInternal(e) {
				subs = append(subs, e)
			}
		}
	}

	sendNotifications("comment_notification", subs,
		map[string]interface{}{
			"Comment": c,
			"Bug":     b,
		})
}

func sendBugNotification(bugid string, fields []string,
	actors, exclude map[string]bool) {

	b, err := getBug(bugid)
	if err != nil {
		log.Printf("Error getting bug %v for bug notification: %v",
			bugid, err)
		return
	}

	// If there's only one actor, exclude that actor from the
	// notifications.  Otherwise, more than one person changed the
	// bugs and everyone should be notified.
	if len(actors) == 1 {
		for k := range actors {
			exclude[k] = true
		}
	}

	to := []string{}
	for _, e := range b.Subscribers {
		if !exclude[e] {
			to = append(to, e)
		}
	}
	acts := []string{}
	for k := range actors {
		acts = append(acts, k)
	}
	sort.Strings(acts)

	sendNotifications("bug_notification", to,
		map[string]interface{}{
			"Fields":       fields,
			"Bug":          b,
			"Actors":       acts,
			"ActorsString": strings.Join(acts, ", "),
		})
}

func bugNotifyDelay(bugid string) chan bugChange {
	rv := make(chan bugChange)

	go func() {
		changes := map[string]bool{}
		exclude := map[string]bool{}
		actors := map[string]bool{}

		t := time.NewTimer(*bugDelay)

		for t != nil {
			select {
			case <-t.C:
				t = nil
			case tc := <-rv:
				if len(tc.fields) == 1 {
					changes[tc.fields[0]] = true
				}
				if tc.exception != "" {
					exclude[tc.exception] = true
				}
				if tc.actor != "" {
					actors[tc.actor] = true
				}
				t.Stop()
				t = time.NewTimer(*bugDelay)
			}
		}

		bugNotifyDelayLock.Lock()
		defer bugNotifyDelayLock.Unlock()
		delete(bugNotifyDelays, bugid)

		fields := []string{}
		for k := range changes {
			fields = append(fields, k)
		}
		sort.Strings(fields)

		sendBugNotification(bugid, fields, actors, exclude)
	}()

	return rv
}

func addBugNotification(bc bugChange) {
	bugNotifyDelayLock.Lock()
	defer bugNotifyDelayLock.Unlock()

	c, ok := bugNotifyDelays[bc.bugid]
	if !ok {
		c = bugNotifyDelay(bc.bugid)
		bugNotifyDelays[bc.bugid] = c
	}
	c <- bc
}

func removeFromList(list []string, needle string) []string {
	rv := []string{}
	for _, s := range list {
		if s != needle {
			rv = append(rv, s)
		}
	}
	return rv
}

func sendBugAssignedNotification(bugid string) {
	b, err := getBug(bugid)
	if err != nil {
		log.Printf("Error getting bug %v for assign notification: %v",
			bugid, err)
		return
	}

	if !strings.Contains(b.Owner, "@") {
		log.Printf("bug %v has no assignee", bugid)
		return
	}

	sendNotifications("assign_notification", []string{b.Owner},
		map[string]interface{}{"Bug": b})
}

func sendTagNotification(bugid, tagName, actor string) {
	b, err := getBug(bugid)
	if err != nil {
		log.Printf("Error getting bug %v for tag notification: %v",
			bugid, err)
		return
	}

	tag := Tag{}
	err = db.Get("tag-"+tagName, &tag)
	if err != nil {
		if !gomemcached.IsNotFound(err) {
			log.Printf("Error sending notification for tag %v: %v",
				tagName, err)
		}
		return
	}

	to := removeFromList(b.Subscribers, actor)

	sendNotifications("tag_notification", to,
		map[string]interface{}{
			"Bug": b,
			"Tag": tagName,
		})
}

func sendBugPingNotification(bp bugPing) {
	sendNotifications("bug_ping", []string{bp.to},
		map[string]interface{}{
			"Bug":       bp.bug,
			"Requester": bp.from,
		})
}

func updateSubscription(bugid, email string, add bool) error {
	return db.Update(bugid, 0, func(current []byte) ([]byte, error) {
		if len(current) == 0 {
			return nil, NotFound
		}
		bug := Bug{}
		err := json.Unmarshal(current, &bug)
		if err != nil {
			return nil, err
		}

		if bug.Type != "bug" {
			return nil, fmt.Errorf("Expected a bug, got %v",
				bug.Type)
		}

		bug.Subscribers = removeFromList(bug.Subscribers, email)
		bug.ModifiedAt = time.Now().UTC()

		if add && emailIsInternal(email) {
			bug.Subscribers = append(bug.Subscribers, email)
		}

		return json.Marshal(bug)
	})
}

func notificationLoop() {
	for {
		select {
		case a := <-attachmentChan:
			sendAttachmentNotification(a)
		case bp := <-pingChan:
			sendBugPingNotification(bp)
		case c := <-commentChan:
			changes_broadcaster.broadcast <- c
			sendCommentNotification(c)
		case bugid := <-assignedChan:
			sendBugAssignedNotification(bugid)
		case c := <-bugChan:
			changes_broadcaster.broadcast <- c
			addBugNotification(c)
		case t := <-tagChan:
			sendTagNotification(t.bugid, t.tag, t.actor)
		}
	}
}
