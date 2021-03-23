package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const (
	adminGroupMembersURI = "https://www.googleapis.com/admin/directory/v1/groups/%s/members"
	adminUserURI         = "https://www.googleapis.com/admin/directory/v1/users/%s?customFieldMask=keys&projection=custom"
)

type googleMemberList struct {
	Members []googleMember `json:"members"`
}

type googleMember struct {
	Email string `json:"email"`
}

type googleAdminUser struct {
	CustomSchemas googleCustomSchema `json:"customSchemas"`
}

type googleCustomSchema struct {
	Keys googleKeys `json:"keys"`
}

type googleKeys struct {
	SSH string `json:"ssh"`
}

type authMap struct {
	LastUpdated string  `json:"lastUpdated"`
	Groups      []group `json:"groups"`
	client      *http.Client
	inputGroups []string
}

type group struct {
	Name string   `json:"name"`
	Keys []string `json:"keys"`
}

func decodeMemberList(body io.Reader) (googleMemberList, error) {
	var memList googleMemberList
	err := json.NewDecoder(body).Decode(&memList)

	return memList, err
}

func (group *group) addSSHKeys(body io.Reader) {
	var adminUser googleAdminUser

	err := json.NewDecoder(body).Decode(&adminUser)
	if err != nil {
		log.Printf("Fail to decode keys %v", err)
	}

	if len(adminUser.CustomSchemas.Keys.SSH) > 0 {
		group.Keys = append(group.Keys, adminUser.CustomSchemas.Keys.SSH)
	}
}

func (am *authMap) groupsFromGoogle() ([]group, error) {
	groups := []group{}
	for _, g := range am.inputGroups {
		grp := group{Name: g, Keys: []string{}}

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf(adminGroupMembersURI, g), nil)
		if err != nil {
			return nil, err
		}

		resp, err := am.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer func() {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}()

		if resp.StatusCode != 200 {
			var body bytes.Buffer
			body.ReadFrom(resp.Body)

			return nil, errors.New(body.String())
		}

		memList, err := decodeMemberList(resp.Body)
		if err != nil {
			return nil, err
		}

		// fetch each user's key + append to map
		for _, m := range memList.Members {
			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf(adminUserURI, m.Email), nil)
			if err != nil {
				return nil, err
			}

			resp, err := am.client.Do(req)
			if err != nil {
				return nil, err
			}
			defer func() {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}()

			if resp.StatusCode != 200 {
				var body bytes.Buffer
				body.ReadFrom(resp.Body)

				return nil, errors.New(body.String())
			}

			grp.addSSHKeys(resp.Body)
		}
		groups = append(groups, grp)
	}
	return groups, nil
}

func (am *authMap) postToAWS() {
	body, _ := json.Marshal(am)

	sess, err := session.NewSession(&aws.Config{})

	if err != nil {
		log.Printf("aws - Failed to create a session %v", err)
		return
	}

	uploader := s3manager.NewUploader(sess)
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(awsBucket),
		Key:         aws.String("authmap"),
		Body:        bytes.NewBuffer(body),
		ContentType: aws.String(http.DetectContentType(body)),
	})
	if err != nil {
		log.Printf("aws - Failed to upload %v", err)
	}
}

func (am *authMap) sync() {
	ticker := time.NewTicker(5 * time.Minute)
	quit := make(chan struct{})

	for {
		select {
		case <-ticker.C:
			gps, err := am.groupsFromGoogle()
			if err == nil {
				syncMutex.Lock()
				am.Groups = gps
				am.LastUpdated = time.Now().String()
				syncMutex.Unlock()
				am.postToAWS()
			} else {
				log.Printf("google - Error building user/group/key map: %v", err)
			}
		case <-quit:
			ticker.Stop()
			return
		}
	}
}
