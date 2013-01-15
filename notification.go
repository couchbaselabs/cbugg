package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/smtp"
	"sort"
	"sync"
	"text/template"
	"time"
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

The following bits of the bug were changed:

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

var commentNotificationTmpl = template.Must(
	template.New("").Parse(commentNotificationText))
var bugNotificationTmpl = template.Must(
	template.New("").Parse(bugChangeNotificationText))

type bugChange struct {
	bugid  string
	fields []string
}

var commentChan = make(chan Comment, 100)
var bugChan = make(chan bugChange, 100)

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
	go bugNotificationLoop()
}

func notifyComment(c Comment) {
	commentChan <- c
}

func notifyBugChange(bugid, field string) {
	bugChan <- bugChange{bugid, []string{field}}
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
	if *mailServer == "" || *mailFrom == "" {
		log.Printf("Email not configured.")
	}

	fields["BaseURL"] = *baseURL
	fields["MailFrom"] = *mailFrom

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

func sendBugNotification(bugid string, fields []string) {
	b, err := getBug(bugid)
	if err != nil {
		log.Printf("Error getting bug %v for bug notification: %v",
			bugid, err)
		return
	}

	sendNotifications(bugNotificationTmpl, b.Subscribers,
		map[string]interface{}{
			"Fields": fields,
			"Bug":    b,
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

		t := time.NewTimer(*bugDelay)

		for t != nil {
			select {
			case <-t.C:
				t = nil
			case tc := <-rv:
				changes[tc.fields[0]] = true
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

		sendBugNotification(bugid, fields)
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
