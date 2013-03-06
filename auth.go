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
	"time"

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

func userFromCookie(cookie string) (User, error) {
	val := browserIdData{}
	err := secureCookie.Decode("user", cookie, &val)
	if err == nil {
		u, err := getUser(val.Email)
		if err != nil {
			u.Id = val.Email
		}
		return u, nil
	}
	return User{}, err
}

func whoami(r *http.Request) User {
	if cookie, err := r.Cookie(AUTH_COOKIE); err == nil {
		u, err := userFromCookie(cookie.Value)
		if err == nil {
			return u
		}
	}
	if ahdr := r.Header.Get("Authorization"); ahdr != "" {
		parts := strings.Split(ahdr, " ")
		if len(parts) < 2 {
			return User{}
		}
		decoded, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return User{}
		}
		userpass := strings.SplitN(string(decoded), ":", 2)

		user, err := getUser(userpass[0])
		if err != nil {
			return User{}
		}

		if user.AuthToken == userpass[1] {
			u, err := getUser(userpass[0])
			if err != nil {
				u.Id = userpass[0]
				u.Type = "user"
			}
			return u
		}
	}
	return User{}
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

	if time.Now().Unix()*1000 >= int64(resdata.Expires) {
		log.Printf("browserId assertion had expired as of %v",
			resdata.Expires)
		showError(w, r, "Browserid assertion is expired", 500)
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

	user, err := getUser(resdata.Email)
	mustEncode(w, map[string]interface{}{
		"email":    resdata.Email,
		"emailmd5": md5string(resdata.Email),
		"prefs":    user.Prefs,
	})
}

func serveLogin(w http.ResponseWriter, r *http.Request) {
	me := whoami(r)

	if me.Id == "" {
		performAuth(w, r)
	} else {
		log.Printf("Reusing existing thing: %v", me.Id)
		mustEncode(w, map[string]interface{}{
			"email":    me.Id,
			"emailmd5": md5string(me.Id),
			"prefs":    me.Prefs,
		})
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
