package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/utilitywarehouse/go-operational/op"
)

var (
	clientID     = os.Getenv("CLIENT_ID")
	clientSecret = os.Getenv("CLIENT_SECRET")
	callbackURL  = os.Getenv("CALLBACK_URL")
	saKeyLoc     = os.Getenv("SA_KEY_LOC")

	scopes = []string{"https://www.googleapis.com/auth/admin.directory.user"}
)

const (
	oauthURL       = "https://accounts.google.com/o/oauth2/auth?redirect_uri=%s&response_type=code&client_id=%s&scope=openid+email+profile&approval_prompt=force&access_type=offline"
	tokenURL       = "https://www.googleapis.com/oauth2/v3/token"
	userInfoURL    = "https://www.googleapis.com/oauth2/v1/userinfo"
	adminUserURL   = "https://www.googleapis.com/admin/directory/v1/users"
	sshKeyPostBody = `{"customSchemas":{"keys":{"ssh":"%s"}}}`
	form           = `<!DOCTYPE html>
<html>
<body>
<form action="/submit">
  *public* ed25519 ssh key (500 chars or less)<br>
  <input type="text" name="key"><br>
  <input type="hidden" name="token" value="%s">
  <input type="submit" value="Submit">
</form>
</body>
</html>
`
)

type UserInfo struct {
	Email string `json:"email"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IdToken      string `json:"id_token"`
}

// Get the id_token and refresh_token from google
func getTokens(clientID, clientSecret, code string) (*TokenResponse, error) {
	val := url.Values{}
	val.Add("grant_type", "authorization_code")
	val.Add("redirect_uri", callbackURL)
	val.Add("client_id", clientID)
	val.Add("client_secret", clientSecret)
	val.Add("code", code)

	resp, err := http.PostForm(tokenURL, val)
	if err != nil {
		return nil, err
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Got: %d calling %s", resp.StatusCode, tokenURL)
	}
	if err != nil {
		return nil, err
	}
	tr := &TokenResponse{}
	err = json.NewDecoder(resp.Body).Decode(tr)
	if err != nil {
		return nil, err
	}
	return tr, nil
}

func getUserEmail(accessToken string) (string, error) {
	uri, _ := url.Parse(userInfoURL)
	q := uri.Query()
	q.Set("alt", "json")
	q.Set("access_token", accessToken)
	uri.RawQuery = q.Encode()
	resp, err := http.Get(uri.String())
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Got: %d calling %s", resp.StatusCode, uri.String())
	}
	if err != nil {
		return "", err
	}
	ui := &UserInfo{}
	err = json.NewDecoder(resp.Body).Decode(ui)
	if err != nil {
		return "", err
	}
	return ui.Email, nil
}

func authenticatedClient() (client *http.Client) {
	data, err := ioutil.ReadFile(saKeyLoc)
	if err != nil {
		log.Fatal(err)
	}
	conf, err := google.JWTConfigFromJSON(data, scopes...)
	conf.Subject = "mdonat@utilitywarehouse.co.uk"
	if err != nil {
		log.Fatal(err)
	}
	return conf.Client(oauth2.NoContext)
}

func googleRedirect() http.Handler {
	redirectURL := fmt.Sprintf(oauthURL, callbackURL, clientID)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectURL, http.StatusFound)
	})
}

func googleCallback() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		tokResponse, err := getTokens(clientID, clientSecret, code)
		if err != nil {
			log.Printf("Error getting tokens: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, err = fmt.Fprintf(w, form, tokResponse.AccessToken)
		if err != nil {
			log.Println("failed to write about response")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
}

func validateKey(key string) error {
	if !strings.HasPrefix(key, "ssh-ed25519") {
		return fmt.Errorf("The key is not a ssh-ed25519 key")
	}
	if len(key) > 500 {
		return fmt.Errorf("The key string must be less then 500 chars")
	}
	return nil
}

func submit(adminClient *http.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.FormValue("key")
		err := validateKey(key)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, err = fmt.Fprintf(w, err.Error())
			return
		}

		token := r.FormValue("token")

		email, err := getUserEmail(token)
		if err != nil {
			log.Printf("Error getting user email: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		userKeysUri := fmt.Sprintf("%s/%s", adminUserURL, email)
		req, err := http.NewRequest(http.MethodPut, userKeysUri, strings.NewReader(fmt.Sprintf(sshKeyPostBody, key)))
		req.Header.Set("content-type", "application/json")

		resp, err := adminClient.Do(req)
		defer func() {
			io.Copy(ioutil.Discard, resp.Body)
			resp.Body.Close()
		}()

		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		body := buf.Bytes()
		if err != nil {
			log.Printf("Failed to make a PUT request to update user: %s in google with ssh key", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if resp.StatusCode != 200 {
			log.Printf("Got: %d calling: %s body: %s", resp.StatusCode, userKeysUri, body)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !bytes.Contains(body, []byte(key)) {
			log.Printf("PUT happened, but didn't return the new value for the ssh key: %s body: %s", key, body)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, err = fmt.Fprintf(w, "Successfully set ssh public key: %s", key)
		if err != nil {
			log.Println("failed to write sucessful user key update response")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
}

func main() {
	m := http.NewServeMux()

	adminClient := authenticatedClient()

	m.Handle("/", googleRedirect())
	m.Handle("/callback", googleCallback())
	m.Handle("/submit", submit(adminClient))

	http.Handle("/__/", op.NewHandler(
		op.NewStatus("Google ssh key manager", "Allows users to set their ssh keys and maintains a list of users/groups/keys in s3.").
			AddOwner("Infrastructure", "#infra").
			ReadyUseHealthCheck(),
	),
	)

	http.Handle("/", m)
	log.Println("Listening on :8080")
	http.ListenAndServe(":8080", nil)
}
