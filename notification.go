package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/dustin/go-humanize"
)

const commentNotificationText = `From: CBugg <{{.MailFrom}}>
To: {{.MailTo}}
Subject: Comment on [{{.Bug.Id}}] {{.Bug.Title}}

{{.Comment.User}} wrote a new comment on "{{.Bug.Title}}":

{{.Comment.Text}}

{{.BaseURL}}{{.Bug.Url}}
`

const bugChangeNotificationText = `From: CBugg <{{.MailFrom}}>
To: {{.MailTo}}
Subject: [{{.Bug.Id}}] {{.Bug.Title}}

The following bits of the bug were changed by {{.ActorsString}}:

{{range .Fields}}* {{.}}
{{ end }}

Here's the new state:

Title:  {{.Bug.Title}}
Status: {{.Bug.Status}}
Owner:  {{.Bug.Owner}}
Tags:   {{range .Bug.Tags}}{{.}} {{end}}

{{.Bug.Description}}


{{.BaseURL}}{{.Bug.Url}}
`

const assignedText = `From: CBugg <{{.MailFrom}}>
To: {{.MailTo}}
Subject: [{{.Bug.Id}}] Assigned to you: {{.Bug.Title}}

The bug "{{.Bug.Title}}" was assigned to you.  You were automatically
subscribed to updates to the bug.

Learn more about it here:

{{.BaseURL}}{{.Bug.Url}}
`

const attachmentNotificationText = `From: CBugg <{{.MailFrom}}>
To: {{.MailTo}}
Subject: Attachment on [{{.Bug.Id}}] {{.Bug.Title}}

There's a new attachment on "{{.Bug.Title}}"

Its name is {{.Att.Filename}} and it's {{.Att.Size | bytes }}

You can grab it here:  {{.BaseURL}}{{.Att.DownloadUrl}}

{{.BaseURL}}{{.Bug.Url}}
`

var commentNotificationTmpl = template.Must(
	template.New("").Parse(commentNotificationText))
var attachmentNotificationTmpl = template.Must(
	template.New("").Funcs(map[string]interface{}{
		"bytes": func(i int64) string {
			return humanize.Bytes(uint64(i))
		},
	}).Parse(attachmentNotificationText))
var bugNotificationTmpl = template.Must(
	template.New("").Parse(bugChangeNotificationText))
var assignedTmpl = template.Must(
	template.New("").Parse(assignedText))

type bugChange struct {
	bugid     string
	actor     string
	fields    []string
	exception string
}

var commentChan = make(chan Comment, 100)
var attachmentChan = make(chan Attachment, 100)
var bugChan = make(chan bugChange, 100)
var assignedChan = make(chan string, 100)

var bugNotifyDelays map[string]chan bugChange
var bugNotifyDelayLock sync.Mutex

// Email configuration

var mailServer = flag.String("smtpserver", "",
	"mail server through which to send notifications")
var mailFrom = flag.String("mailfrom", "",
	"cbugg email address for notifications")
var baseURL = flag.String("baseurl", "http://localhost:8066",
	"base URL of cbugg service")
var bugDelay = flag.Duration("notificationDelay",
	time.Duration(10*time.Second),
	"bug change stabilization delay timer")

func init() {
	bugNotifyDelays = make(map[string]chan bugChange)

	go commentNotificationLoop()
	go attachmentNotificationLoop()
	go bugNotificationLoop()
	go bugAssignmentNotificationLoop()
}

func notifyComment(c Comment) {
	commentChan <- c
}

func notifyAttachment(a Attachment) {
	attachmentChan <- a
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

func sendEmail(to string, body []byte) error {
	c, err := smtp.Dial(*mailServer)
	if err != nil {
		return err
	}
	if err = c.Mail(*mailFrom); err != nil {
		return err
	}
	if err = c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(body)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	return c.Quit()
}

func sendNotifications(t *template.Template,
	subs []string, fields map[string]interface{}) {

	fields["BaseURL"] = *baseURL
	fields["MailFrom"] = *mailFrom

	if *mailServer == "" || *mailFrom == "" {
		log.Printf("Email not configured, would have sent this:")
		fields["MailTo"] = "someone@example.com"
		t.Execute(os.Stderr, fields)

		return
	}

	for _, to := range subs {
		buf := &bytes.Buffer{}

		fields["MailTo"] = to
		err := t.Execute(buf, fields)

		if err != nil {
			log.Printf("Error building mail body: %v", err)
			continue
		}

		err = sendEmail(to, buf.Bytes())

		if err != nil {
			log.Printf("Error sending email: %v", err)
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

	sendNotifications(attachmentNotificationTmpl, b.Subscribers,
		map[string]interface{}{
			"Att": a,
			"Bug": b,
		})
}

func attachmentNotificationLoop() {
	for a := range attachmentChan {
		sendAttachmentNotification(a)
	}
}

func sendCommentNotification(c Comment) {
	b, err := getBug(c.BugId)
	if err != nil {
		log.Printf("Error getting bug %v for comment notification: %v",
			c.BugId, err)
		return
	}

	sendNotifications(commentNotificationTmpl, b.Subscribers,
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

	sendNotifications(bugNotificationTmpl, to,
		map[string]interface{}{
			"Fields":       fields,
			"Bug":          b,
			"Actors":       acts,
			"ActorsString": strings.Join(acts, ", "),
		})
}

func commentNotificationLoop() {
	for c := range commentChan {
		sendCommentNotification(c)
	}
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

func bugNotificationLoop() {
	for c := range bugChan {
		addBugNotification(c)
	}
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

	sendNotifications(assignedTmpl, []string{b.Owner},
		map[string]interface{}{"Bug": b})
}

func bugAssignmentNotificationLoop() {
	for bugid := range assignedChan {
		sendBugAssignedNotification(bugid)
	}
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

		if add {
			bug.Subscribers = append(bug.Subscribers, email)
		}

		return json.Marshal(bug)
	})
}
