package main

import (
	"bytes"
	"flag"
	"io"
	"net/http"
	"net/mail"
	"net/smtp"
	"os"
	"path/filepath"
	"text/template"

	"github.com/dustin/go-humanize"
)

var templates = template.Must(initTemplates())

// Email configuration

var mailServer = flag.String("smtpserver", "",
	"mail server through which to send notifications")
var mailFrom = flag.String("mailfrom", "",
	"cbugg email address for notifications")
var baseURL = flag.String("baseurl", "http://localhost:8066",
	"base URL of cbugg service")
var replyToDom = flag.String("ird", "cbugg.hq.couchbase.com",
	"In-Reply-To domain to use (arbitrary string)")

var defaultHeaders = mail.Header{
	"Content-Type": []string{"text/plain; charset=utf-8"},
	"From":         []string{"CBugg <{{.MailFrom}}>"},
	"In-Reply-To":  []string{"<{{.Bug.Id}}.{{.InReplyToDom}}>"},
	"To":           []string{"{{.MailTo}}"},
}

func initTemplates() (*template.Template, error) {
	t := template.New("").Funcs(map[string]interface{}{
		"bytes": func(i int64) string {
			return humanize.Bytes(uint64(i))
		},
		"shortName": func(s string) string {
			return Email(s).shortEmail()
		},
	})

	files, err := filepath.Glob("templates/*")
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		basename := filepath.Base(f)

		err := prepTemplate(t, f, basename)
		if err != nil {
			return nil, err
		}
	}

	return t, nil
}

func prepTemplate(parent *template.Template, fn, base string) error {
	t := parent.New(base)

	f, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer f.Close()

	msg, err := mail.ReadMessage(f)
	if err != nil {
		return err
	}

	for k, v := range defaultHeaders {
		if msg.Header.Get(k) == "" {
			msg.Header[k] = v
		}
	}

	data := &bytes.Buffer{}

	// This is the only place I could find this method.  :/
	http.Header(msg.Header).Write(data)
	data.Write([]byte{'\r', '\n'})

	_, err = io.Copy(data, msg.Body)
	if err != nil {
		return err
	}

	_, err = t.Parse(data.String())
	return err
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
