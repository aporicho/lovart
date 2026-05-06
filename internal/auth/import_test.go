package auth

import "testing"

func TestParseJSONSupportsFlatAndNestedCredentials(t *testing.T) {
	flat, err := ParseJSON([]byte(`{"cookie":"c","token":"t","csrf":"x","project_id":"p","cid":"cid"}`))
	if err != nil {
		t.Fatalf("ParseJSON flat: %v", err)
	}
	if flat.Cookie != "c" || flat.Token != "t" || flat.CSRF != "x" || flat.ProjectID != "p" || flat.CID != "cid" {
		t.Fatalf("flat session = %#v", flat)
	}

	nested, err := ParseJSON([]byte(`{"headers":{"Cookie":"c2","token":"t2","x-csrf-token":"x2"},"ids":{"projectId":"p2","webid":"cid2"}}`))
	if err != nil {
		t.Fatalf("ParseJSON nested: %v", err)
	}
	if nested.Cookie != "c2" || nested.Token != "t2" || nested.CSRF != "x2" || nested.ProjectID != "p2" || nested.CID != "cid2" {
		t.Fatalf("nested session = %#v", nested)
	}
}

func TestParseCurlAndHeaders(t *testing.T) {
	curl := []byte(`curl 'https://www.lovart.ai/api' -H 'cookie: sid=abc' -H 'token: tok' --data '{"projectId":"p","cid":"cid"}'`)
	session, err := ParseCurl(curl)
	if err != nil {
		t.Fatalf("ParseCurl: %v", err)
	}
	if session.Cookie != "sid=abc" || session.Token != "tok" || session.ProjectID != "p" || session.CID != "cid" {
		t.Fatalf("curl session = %#v", session)
	}

	headers := []byte("Cookie: sid=def\nX-CSRF-Token: csrf\n")
	session, err = ParseHeaders(headers)
	if err != nil {
		t.Fatalf("ParseHeaders: %v", err)
	}
	if session.Cookie != "sid=def" || session.CSRF != "csrf" {
		t.Fatalf("header session = %#v", session)
	}
}
