package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const (
	adminGroupMembersURI = "https://www.googleapis.com/admin/directory/v1/groups/%s/members"
	adminUserURI         = "https://www.googleapis.com/admin/directory/v1/users/%s?customFieldMask=keys&projection=custom"
)

type GoogleMemberList struct {
	Members []GoogleMember `json:"members"`
}

type GoogleMember struct {
	Email string `json:"email"`
}

type GoogleAdminUser struct {
	CustomSchemas GoogleCustomSchema `json:"customSchemas"`
}

type GoogleCustomSchema struct {
	Keys GoogleKeys `json:"keys"`
}

type GoogleKeys struct {
	SSH string `json:"ssh"`
}

type AuthMap struct {
	LastUpdated string  `json:"lastUpdated"`
	Groups      []Group `json:"groups"`
	client      *http.Client
	inputGroups []string
}

type Group struct {
	Name string   `json:"name"`
	Keys []string `json:"keys"`
}

func decodeMemberList(body io.Reader) (GoogleMemberList, error) {
	var memList GoogleMemberList
	err := json.NewDecoder(body).Decode(&memList)

	return memList, err
}

func (group *Group) addSSHKeys(body io.Reader) {
	var adminUser GoogleAdminUser

	err := json.NewDecoder(body).Decode(&adminUser)
	if err != nil {
		log.Printf("Fail to decode keys %v", err)
	}

	if len(adminUser.CustomSchemas.Keys.SSH) > 0 {
		group.Keys = append(group.Keys, adminUser.CustomSchemas.Keys.SSH)
	}
}

func (am *AuthMap) groupsFromGoogle() ([]Group, error) {
	groups := []Group{}
	for _, g := range am.inputGroups {
		group := Group{Name: g, Keys: []string{}}

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf(adminGroupMembersURI, g), nil)
		if err != nil {
			return nil, err
		}

		resp, err := am.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer func() {
			io.Copy(ioutil.Discard, resp.Body)
			resp.Body.Close()
		}()

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
				io.Copy(ioutil.Discard, resp.Body)
				resp.Body.Close()
			}()

			group.addSSHKeys(resp.Body)
		}
		groups = append(groups, group)
	}
	return groups, nil
}

func (am *AuthMap) postToAWS() {
	body, _ := json.Marshal(am)

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("eu-west-1"),
		Credentials: credentials.NewStaticCredentials(awsAccessKey, awsSecretKey, ""),
	})

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

func (am *AuthMap) sync() {
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
