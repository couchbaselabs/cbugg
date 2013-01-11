package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/securecookie"
)

const (
	BROWSERID_ENDPOINT = "https://verifier.login.persona.org/verify"
)

type browserIdData struct {
	Status   string
	Reason   string
	Email    string
	Audience string
	Expires  uint64
	Issuer   string
}

var secureCookie *securecookie.SecureCookie

func initSecureCookie(hashKey []byte) {
	secureCookie = securecookie.New(hashKey, nil)
}

func serveLogin(w http.ResponseWriter, r *http.Request) {
	assertion := r.FormValue("assertion")
	if assertion == "" {
		showError(w, r, "No assertion requested.", 400)
		return
	}
	data := map[string]string{
		"assertion": assertion,
		"audience":  r.FormValue("audience"),
	}

	body, err := json.Marshal(&data)
	if err != nil {
		showError(w, r, "Error encoding request: "+err.Error(), 500)
		return
	}

	req, err := http.NewRequest("POST", BROWSERID_ENDPOINT,
		bytes.NewReader(body))
	if err != nil {
		panic(err)
	}

	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)

	if err != nil {
		showError(w, r, "Error transmitting request: "+err.Error(), 500)
		return
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		showError(w, r, "Invalid response code from browserid: "+res.Status, 500)
		return
	}

	resdata := browserIdData{}

	d := json.NewDecoder(res.Body)
	err = d.Decode(&resdata)
	if err != nil {
		showError(w, r, "Error decoding browserid response: "+err.Error(), 500)
		return
	}

	if resdata.Status != "okay" {
		showError(w, r, "Browserid status was not OK: "+
			resdata.Status+"/"+resdata.Reason, 500)
		return
	}

	encoded, err := secureCookie.Encode("user", resdata)
	if err != nil {
		showError(w, r, "Couldn't encode cookie: "+err.Error(), 500)
		return
	}

	cookie := &http.Cookie{
		Name:  "cookie-name",
		Value: encoded,
		Path:  "/",
	}
	http.SetCookie(w, cookie)

	h := md5.New()
	h.Write([]byte(resdata.Email))
	emailmd5 := hex.EncodeToString(h.Sum(nil))

	log.Printf("Logged in %v", resdata.Email)

	w.Header().Set("Content-Type", "application/json")
	mustEncode(w, map[string]string{
		"email":    resdata.Email,
		"emailmd5": emailmd5,
	})
}
