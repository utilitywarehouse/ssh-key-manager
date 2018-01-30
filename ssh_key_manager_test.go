package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"testing"
)

func encodeToReader(i interface{}) io.ReadCloser {
	b := new(bytes.Buffer)
	enc := json.NewEncoder(b)
	enc.Encode(i)

	return ioutil.NopCloser(strings.NewReader(b.String()))
}

func TestDecodeMemberList(t *testing.T) {
	gMember1 := &GoogleMember{"member1@uw.co.uk"}
	gMember2 := &GoogleMember{"member2@uw.co.uk"}
	gMemberList := &GoogleMemberList{
		Members: []GoogleMember{*gMember1, *gMember2},
	}
	r := encodeToReader(*gMemberList)

	memberList, err := decodeMemberList(r)
	if err != nil {
		log.Fatal(err)
	}

	if len(memberList.Members) != 2 {
		t.Error("Unexpected MemberList", memberList.Members)
	}
}

func TestAddSSHKeys(t *testing.T) {
	group := &Group{Name: "dummy group"}

	emptyKey := &GoogleKeys{SSH: ""}
	schema := &GoogleCustomSchema{Keys: *emptyKey}
	adminUser := &GoogleAdminUser{CustomSchemas: *schema}
	r := encodeToReader(*adminUser)

	group.addSSHKeys(r)
	if len(group.Keys) > 0 {
		t.Error("empty key!", group.Keys)
	}

	dummyKey := &GoogleKeys{SSH: "dummy ssh key"}
	schema = &GoogleCustomSchema{Keys: *dummyKey}
	adminUser = &GoogleAdminUser{CustomSchemas: *schema}
	r = encodeToReader(*adminUser)

	group.addSSHKeys(r)
	if len(group.Keys) == 0 {
		t.Error("Key not added", group.Keys)
	}
}

// Mock http Do function for requesting keys from googleapis
type mockTransport struct{}

func newMockTransport() http.RoundTripper {
	return &mockTransport{}
}
func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	response := &http.Response{
		Header:     make(http.Header),
		Request:    req,
		StatusCode: http.StatusOK,
	}

	response.Header.Set("Content-Type", "application/json")
	if req.URL.String() == "https://www.googleapis.com/admin/directory/v1/groups/ingroup1/members" {
		gMember1 := &GoogleMember{"member1@uw.co.uk"}
		gMember2 := &GoogleMember{"member2@uw.co.uk"}
		gMemberList := &GoogleMemberList{
			Members: []GoogleMember{*gMember1, *gMember2},
		}
		response.Body = encodeToReader(*gMemberList)
	} else if req.URL.String() == "https://www.googleapis.com/admin/directory/v1/users/member1@uw.co.uk?customFieldMask=keys&projection=custom" {
		dummyKey := &GoogleKeys{SSH: "dummy ssh key"}
		schema := &GoogleCustomSchema{Keys: *dummyKey}
		adminUser := &GoogleAdminUser{CustomSchemas: *schema}
		response.Body = encodeToReader(*adminUser)
	} else if req.URL.String() == "https://www.googleapis.com/admin/directory/v1/users/member2@uw.co.uk?customFieldMask=keys&projection=custom" {
		emptyKey := &GoogleKeys{SSH: ""}
		schema := &GoogleCustomSchema{Keys: *emptyKey}
		adminUser := &GoogleAdminUser{CustomSchemas: *schema}
		response.Body = encodeToReader(*adminUser)
	}
	return response, nil
}

func TestGoogleGroupsIgnoreEmptyKeys(t *testing.T) {

	client := http.DefaultClient
	client.Transport = newMockTransport()

	am := &AuthMap{client: client}
	inputGroups := []string{"ingroup1"}
	am.inputGroups = inputGroups

	groups, err := am.groupsFromGoogle()
	if err != nil {
		log.Fatal(err)
	}
	if len(groups) > 1 {
		t.Error("Got more groups than input")
	}
	if groups[0].Name != "ingroup1" {
		t.Error("Got different group than requested")
	}
	if len(groups[0].Keys) > 1 {
		t.Error("Got more keys (empty included)", groups[0].Keys)
	}

}
