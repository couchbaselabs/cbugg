package main

import (
	"bytes"
	"flag"
	"log"
	"net/smtp"
	"text/template"
)

const commentNotificationText = `From: CBugg <{{.MailFrom}}>
To: {{.MailTo}}
Subject: Comment on [{{.Bug.Id}}] {{.Bug.Title}}

{{.Comment.User}} wrote a new comment on "{{.Bug.Title}}":

{{.Comment.Text}}

{{.BaseURL}}{{.Bug.Url}}
`

var commentNotificationTmpl = template.Must(
	template.New("").Parse(commentNotificationText))

var commentChan = make(chan Comment, 100)

// Email configuration

var mailServer = flag.String("smtpserver", "",
	"mail server through which to send notifications")
var mailFrom = flag.String("mailfrom", "",
	"cbugg email address for notifications")
var baseURL = flag.String("baseurl", "http://localhost:8066",
	"base URL of cbugg service")

func init() {
	go commentNotificationLoop()
}

func notifyComment(c Comment) {
	commentChan <- c
}

func sendEmail(to string, body []byte) error {
	c, err := smtp.Dial(*mailServer)
	if err != nil {
		return err
	}
	if err := c.Hello("localhost"); err != nil {
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

func sendCommentNotification(c Comment) {
	if *mailServer == "" || *mailFrom == "" {
		log.Printf("Email not configured.")
	}

	b, err := getBug(c.BugId)
	if err != nil {
		log.Printf("Error getting bug %v for comment notification: %v",
			c.BugId, err)
		return
	}

	for _, to := range b.Subscribers {
		buf := &bytes.Buffer{}

		err = commentNotificationTmpl.Execute(buf, map[string]interface{}{
			"BaseURL":  *baseURL,
			"MailFrom": *mailFrom,
			"MailTo":   to,
			"Comment":  c,
			"Bug":      b,
		})

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

func commentNotificationLoop() {
	for c := range commentChan {
		sendCommentNotification(c)
	}
}
