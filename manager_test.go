package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"testing"
)

func encodeToReader(i interface{}) io.ReadCloser {
	var b bytes.Buffer

	json.NewEncoder(&b).Encode(i)
	return ioutil.NopCloser(&b)
}

func TestDecodeMemberList(t *testing.T) {
	gMember1 := googleMember{"member1@uw.co.uk"}
	gMember2 := googleMember{"member2@uw.co.uk"}
	gMemberList := googleMemberList{
		Members: []googleMember{gMember1, gMember2},
	}
	r := encodeToReader(gMemberList)

	memberList, err := decodeMemberList(r)
	if err != nil {
		t.Fatal(err)
	}

	if len(memberList.Members) != 2 {
		t.Error("Unexpected MemberList", memberList.Members)
	}
}

func TestAddSSHKeys(t *testing.T) {
	dg := group{Name: "dummy group"}

	emptyKey := googleKeys{SSH: ""}
	schema := googleCustomSchema{Keys: emptyKey}
	adminUser := googleAdminUser{CustomSchemas: schema}
	r := encodeToReader(adminUser)

	dg.addSSHKeys(r)
	if len(dg.Keys) > 0 {
		t.Error("empty key!", dg.Keys)
	}

	dummyKey := googleKeys{SSH: "dummy ssh key"}
	schema = googleCustomSchema{Keys: dummyKey}
	adminUser = googleAdminUser{CustomSchemas: schema}
	r = encodeToReader(adminUser)

	dg.addSSHKeys(r)
	if len(dg.Keys) == 0 {
		t.Error("Key not added", dg.Keys)
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
		gMember1 := &googleMember{"member1@uw.co.uk"}
		gMember2 := &googleMember{"member2@uw.co.uk"}
		gMemberList := &googleMemberList{
			Members: []googleMember{*gMember1, *gMember2},
		}
		response.Body = encodeToReader(*gMemberList)
	} else if req.URL.String() == "https://www.googleapis.com/admin/directory/v1/users/member1@uw.co.uk?customFieldMask=keys&projection=custom" {
		dummyKey := &googleKeys{SSH: "dummy ssh key"}
		schema := &googleCustomSchema{Keys: *dummyKey}
		adminUser := &googleAdminUser{CustomSchemas: *schema}
		response.Body = encodeToReader(*adminUser)
	} else if req.URL.String() == "https://www.googleapis.com/admin/directory/v1/users/member2@uw.co.uk?customFieldMask=keys&projection=custom" {
		emptyKey := &googleKeys{SSH: ""}
		schema := &googleCustomSchema{Keys: *emptyKey}
		adminUser := &googleAdminUser{CustomSchemas: *schema}
		response.Body = encodeToReader(*adminUser)
	}
	return response, nil
}

func TestGoogleGroupsIgnoreEmptyKeys(t *testing.T) {

	client := http.DefaultClient
	client.Transport = newMockTransport()

	am := &authMap{client: client}
	inputGroups := []string{"ingroup1"}
	am.inputGroups = inputGroups

	groups, err := am.groupsFromGoogle()
	if err != nil {
		t.Fatal(err)
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
