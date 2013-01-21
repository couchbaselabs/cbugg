package main

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/securecookie"
)

const (
	BROWSERID_ENDPOINT = "https://verifier.login.persona.org/verify"
	AUTH_COOKIE        = "cbugger"
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

func whoami(r *http.Request) string {
	if cookie, err := r.Cookie(AUTH_COOKIE); err == nil {
		val := browserIdData{}
		if err = secureCookie.Decode("user", cookie.Value, &val); err == nil {
			// TODO: Check expiration
			return val.Email
		}
	}
	if ahdr := r.Header.Get("Authorization"); ahdr != "" {
		parts := strings.Split(ahdr, " ")
		if len(parts) < 2 {
			return ""
		}
		decoded, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
		userpass := strings.SplitN(string(decoded), ":", 2)

		user, err := getUser(userpass[0])
		if err != nil {
			return ""
		}

		if user.AuthToken == userpass[1] {
			return userpass[0]
		}
	}
	return ""
}

func md5string(i string) string {
	h := md5.New()
	h.Write([]byte(i))
	return hex.EncodeToString(h.Sum(nil))
}

func performAuth(w http.ResponseWriter, r *http.Request) {
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
		Name:  AUTH_COOKIE,
		Value: encoded,
		Path:  "/",
	}
	http.SetCookie(w, cookie)

	log.Printf("Logged in %v", resdata.Email)

	mustEncode(w, map[string]string{
		"email":    resdata.Email,
		"emailmd5": md5string(resdata.Email),
	})
}

func serveLogin(w http.ResponseWriter, r *http.Request) {
	email := whoami(r)

	if email == "" {
		performAuth(w, r)
	} else {
		log.Printf("Reusing existing thing: %v", email)
		mustEncode(w, map[string]string{
			"email":    email,
			"emailmd5": md5string(email),
		})
		return
	}
}

func serveLogout(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{
		Name:  AUTH_COOKIE,
		Value: "",
		Path:  "/",
	}

	http.SetCookie(w, cookie)
}
