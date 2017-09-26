package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	adminGroupMembersURI = "https://www.googleapis.com/admin/directory/v1/groups/%s/members"
	adminUserURI         = "https://www.googleapis.com/admin/directory/v1/users/%s?customFieldMask=keys&projection=custom"
)

// Google group members
type GoogleMemberList struct {
	Members []GoogleMember `json:"members"`
}

type GoogleMember struct {
	Email string `json:"email"`
}

// Google admin user
type GoogleAdminUser struct {
	CustomSchemas GoogleCustomSchema `json:"customSchemas"`
}

type GoogleCustomSchema struct {
	Keys GoogleKeys `json:"keys"`
}

type GoogleKeys struct {
	SSH string `json:"ssh"`
}

type Group struct {
	Name string   `json:"name"`
	Keys []string `json:"keys"`
}

func authmap(adminClient *http.Client, groups []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rtnGroups []Group
		for _, g := range groups {
			var memList GoogleMemberList
			group := Group{Name: g, Keys: []string{}}
			req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf(adminGroupMembersURI, g), nil)
			resp, _ := adminClient.Do(req)
			buf := new(bytes.Buffer)
			buf.ReadFrom(resp.Body)
			body := buf.Bytes()

			json.Unmarshal(body, &memList)

			// fetch each user's key + append to map
			for _, m := range memList.Members {
				var adminUser GoogleAdminUser
				req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf(adminUserURI, m.Email), nil)
				resp, _ := adminClient.Do(req)
				buf := new(bytes.Buffer)
				buf.ReadFrom(resp.Body)
				body := buf.Bytes()
				json.Unmarshal(body, &adminUser)

				group.Keys = append(group.Keys, adminUser.CustomSchemas.Keys.SSH)
			}
			rtnGroups = append(rtnGroups, group)
		}
		bd, _ := json.Marshal(rtnGroups)
		fmt.Fprintf(w, "%s", bd)
		return
	})
}
