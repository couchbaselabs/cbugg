package main

import (
	"flag"
	"net/smtp"
	"text/template"

	"github.com/dustin/go-humanize"
)

var templates *template.Template

// Email configuration

var mailServer = flag.String("smtpserver", "",
	"mail server through which to send notifications")
var mailFrom = flag.String("mailfrom", "",
	"cbugg email address for notifications")
var baseURL = flag.String("baseurl", "http://localhost:8066",
	"base URL of cbugg service")

func init() {
	t := template.New("").Funcs(map[string]interface{}{
		"bytes": func(i int64) string {
			return humanize.Bytes(uint64(i))
		},
		"shortName": func(s string) string {
			return Email(s).shortEmail()
		},
	})

	templates = template.Must(t.ParseGlob("templates/*"))
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
