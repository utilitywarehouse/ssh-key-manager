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
	"sync"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"

	"github.com/utilitywarehouse/go-operational/op"
)

var (
	googleClientID     = os.Getenv("SKM_CLIENT_ID")
	googleClientSecret = os.Getenv("SKM_CLIENT_SECRET")
	googleCallbackURL  = os.Getenv("SKM_CALLBACK_URL")
	awsAccessKey       = os.Getenv("SKM_AWS_ACCESS_KEY_ID")
	awsSecretKey       = os.Getenv("SKM_AWS_SECRET_ACCESS_KEY")
	awsBucket          = os.Getenv("SKM_AWS_BUCKET")
	saKeyLoc           = os.Getenv("SKM_SA_KEY_LOC")
	groups             = os.Getenv("SKM_GROUPS")

	scopes = []string{"https://www.googleapis.com/auth/admin.directory.user", "https://www.googleapis.com/auth/admin.directory.group.member.readonly"}

	syncMutex = &sync.RWMutex{}
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
  *public* ed25519 or ecdsa ssh key (500 chars or less)<br>
  <input type="text" name="key"><br>
  <input type="hidden" name="token" value="%s">
  <input type="submit" value="Submit">
</form>
</body>
</html>
`
)

type userInfo struct {
	Email string `json:"email"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
}

// Get the id_token and refresh_token from google
func getTokens(clientID, clientSecret, code string) (*tokenResponse, error) {
	val := url.Values{}
	val.Add("grant_type", "authorization_code")
	val.Add("redirect_uri", googleCallbackURL)
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
		return nil, fmt.Errorf("google - unexpected response: %d calling %s", resp.StatusCode, tokenURL)
	}
	if err != nil {
		return nil, err
	}
	tr := &tokenResponse{}
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
		return "", fmt.Errorf("google - unexpected response: %d calling %s", resp.StatusCode, uri.String())
	}
	if err != nil {
		return "", err
	}
	ui := &userInfo{}
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
	return conf.Client(context.TODO())
}

func googleRedirect() http.Handler {
	redirectURL := fmt.Sprintf(oauthURL, googleCallbackURL, googleClientID)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectURL, http.StatusFound)
	})
}

func googleCallback() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		tokResponse, err := getTokens(googleClientID, googleClientSecret, code)
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
	if !strings.HasPrefix(key, "ssh-ed25519") && !strings.HasPrefix(key, "ecdsa-sha2-nistp256") {
		return fmt.Errorf("The key must be [ssh-ed25519|ecdsa-sha2-nistp256] key")
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
			fmt.Fprintf(w, err.Error())
			return
		}

		token := r.FormValue("token")

		email, err := getUserEmail(token)
		if err != nil {
			log.Printf("Error getting user email: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// strip the comment and replace with email
		keyParts := strings.Split(key, " ")
		key = keyParts[0] + " " + keyParts[1] + " " + email

		userKeysURI := fmt.Sprintf("%s/%s", adminUserURL, email)
		req, _ := http.NewRequest(http.MethodPut, userKeysURI, strings.NewReader(fmt.Sprintf(sshKeyPostBody, key)))
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
			log.Printf("Got: %d calling: %s body: %s", resp.StatusCode, userKeysURI, body)
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

func authMapPage(am *authMap) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		syncMutex.RLock()
		defer syncMutex.RUnlock()
		enc := json.NewEncoder(w)
		enc.Encode(am)
	})
}

func main() {
	adminClient := authenticatedClient()
	groups := strings.Split(groups, ",")
	am := &authMap{client: adminClient, inputGroups: groups}
	go am.sync()

	m := http.NewServeMux()
	m.Handle("/", googleRedirect())
	m.Handle("/callback", googleCallback())
	m.Handle("/submit", submit(adminClient))
	m.Handle("/authmap", authMapPage(am))
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
