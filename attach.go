package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

var cbfsUrl = flag.String("cbfs", "",
	"URL in CBFS to store attachments")

var alphabet []byte

func init() {
	letters := "ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789"

	for _, r := range letters {
		alphabet = append(alphabet, byte(r))
	}
}

func randstring(l int) string {
	stuff := make([]byte, l)
	bytesread, err := rand.Read(stuff)
	if err != nil {
		panic(err)
	}
	if bytesread != l {
		panic("short read")
	}

	for i := range stuff {
		stuff[i] = alphabet[int(stuff[i])%len(alphabet)]
	}
	return string(stuff)
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(b []byte) (int, error) {
	n, err := c.r.Read(b)
	if n > 0 {
		c.n += int64(n)
	}
	return n, err
}

func serveFileUpload(w http.ResponseWriter, r *http.Request) {
	if *cbfsUrl == "" {
		showError(w, r, "attachment storage is not configured", 500)
		return
	}

	bugid := mux.Vars(r)["bugid"]
	me := whoami(r)
	if _, err := getBugOrDisplayErr(bugid, me, w, r); err != nil {
		return
	}

	f, fh, err := r.FormFile("uploadedFile")
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}
	defer f.Close()

	attid := randstring(8)
	dest := *cbfsUrl + bugid + "/" + attid + "/" + fh.Filename

	cr := &countingReader{r: f}

	req, err := http.NewRequest("PUT", dest, cr)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	req.Header.Set("Content-Type", fh.Header.Get("Content-Type"))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		showError(w, r, res.Status, 500)
		return
	}

	att := Attachment{
		Id:          bugid + "-" + attid,
		BugId:       bugid,
		Type:        "attachment",
		Url:         dest,
		Size:        cr.n,
		ContentType: fh.Header.Get("Content-Type"),
		Filename:    fh.Filename,
		User:        me.Id,
		CreatedAt:   time.Now().UTC(),
	}

	err = db.Set("att-"+bugid+"-"+attid, 0, &att)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	log.Printf("Attached %v -> %v", attid, dest)

	notifyAttachment(att)

	w.WriteHeader(200)
	mustEncode(w, map[string]interface{}{
		"id":           "att-" + att.Id,
		"user":         Email(att.User),
		"filename":     att.Filename,
		"content_type": att.ContentType,
		"created_at":   att.CreatedAt,
		"size":         att.Size,
	})
}

func serveAttachmentList(w http.ResponseWriter, r *http.Request) {
	bugid := mux.Vars(r)["bugid"]
	me := whoami(r)
	if _, err := getBugOrDisplayErr(bugid, me, w, r); err != nil {
		return
	}

	args := map[string]interface{}{
		"stale":        false,
		"start_key":    []interface{}{bugid},
		"end_key":      []interface{}{bugid, map[string]string{}},
		"include_docs": true,
	}

	viewRes := struct {
		Rows []struct {
			Id  string
			Key []string
			Doc struct {
				Json struct {
					Filename    string
					ContentType string `json:"content_type"`
					User        Email
					Size        int64
				}
			}
		}
	}{}

	err := db.ViewCustom("cbugg", "attachments", args, &viewRes)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	type outT struct {
		Id          string `json:"id"`
		User        Email  `json:"user"`
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
		Size        int64  `json:"size"`
		Timestamp   string `json:"created_at"`
	}

	out := []outT{}

	for _, r := range viewRes.Rows {
		out = append(out, outT{
			r.Id,
			r.Doc.Json.User,
			r.Doc.Json.Filename,
			r.Doc.Json.ContentType,
			r.Doc.Json.Size,
			r.Key[1],
		})
	}

	mustEncode(w, out)
}

func serveAttachment(w http.ResponseWriter, r *http.Request) {
	attid := mux.Vars(r)["attid"]
	bugid := mux.Vars(r)["bugid"]
	me := whoami(r)
	if _, err := getBugOrDisplayErr(bugid, me, w, r); err != nil {
		return
	}

	att := Attachment{}
	err := db.Get(attid, &att)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	if att.Type != "attachment" {
		showError(w, r, "not an attachment", 500)
		return
	}

	res, err := http.Get(att.Url)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		showError(w, r, res.Status, 500)
		return
	}

	w.Header().Set("Content-Type", res.Header.Get("Content-Type"))
	w.Header().Set("Content-Length", fmt.Sprintf("%v", res.ContentLength))
	w.Header().Set("Content-Disposition", "attachment")

	_, err = io.Copy(w, res.Body)
	if err != nil {
		log.Printf("Error sending attachment: %v", err)
	}
}

func serveHeadAttachment(w http.ResponseWriter, r *http.Request) {
	attid := mux.Vars(r)["attid"]
	bugid := mux.Vars(r)["bugid"]
	me := whoami(r)
	if _, err := getBugOrDisplayErr(bugid, me, w, r); err != nil {
		return
	}

	att := Attachment{}
	err := db.Get(attid, &att)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	if att.Type != "attachment" {
		showError(w, r, "not an attachment", 500)
		return
	}

	res, err := http.Head(att.Url)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		showError(w, r, res.Status, 500)
		return
	}

	w.Header().Set("Content-Type", res.Header.Get("Content-Type"))
	w.Header().Set("Content-Length", fmt.Sprintf("%v", res.ContentLength))
	w.Header().Set("Content-Disposition", "attachment")
}

func deleteAttachmentFile(url string) error {
	log.Printf("Deleting attachment file at %v", url)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		log.Printf("Error creating DELETE request: %v", err)
		return err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error sending DELETE request to cbfs: %v", err)
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 204 {
		log.Printf("Deletion failed: %v", res.Status)
		return fmt.Errorf("Deletion failed: %v", res.Status)
	}
	return nil
}

func serveDeleteAttachment(w http.ResponseWriter, r *http.Request) {
	attid := mux.Vars(r)["attid"]
	me := whoami(r)

	att := Attachment{}
	err := db.Get(attid, &att)
	if err != nil {
		showError(w, r, err.Error(), 500)
		return
	}

	if att.Type != "attachment" {
		showError(w, r, "not an attachment", 500)
		return
	}

	if !(me.Admin || att.User == me.Id) {
		showError(w, r, "not your attachment", 400)
		return
	}

	// First, kill the reference in cbugg
	err = db.Delete(attid)
	if err != nil {
		showError(w, r, err.Error(), 400)
		return
	}

	// Nothing below is fatal.
	w.WriteHeader(204)

	// Then delete it from CBFS
	go deleteAttachmentFile(att.Url)
}
